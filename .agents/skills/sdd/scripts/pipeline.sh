#!/usr/bin/env sh
# Spec-Driven Dev Pipeline — state machine (POSIX sh, zero dependencies)
# Usage: sh pipeline.sh [--feature <name>] <command> [args]
#
# Shell compatibility: requires sh with `local` support (bash, dash, ash, zsh).
#
# Global flags:
#   --feature <name>      Specify which feature to operate on (required when
#                         multiple pipelines are active simultaneously)
#
# Commands (run `sh pipeline.sh help` for full details):
#   init <name>                                     Start a new pipeline
#   status / approve / artifact [path]              Core phase flow
#   task-init/task/task-next/tasks/task-reset       Implementation task tracking
#   inject <phase> <path>                           Inject artifact, skip to phase
#   revisions [phase] / history                     Inspect revisions / features
#   abandon [feature]                               Abandon an active pipeline
#   config-check                                    Validate .spec/config.yaml
#   doctor                                          Diagnose environment / setup
#   docs-check                                      Project docs status (JSON)
#   docs-init/next/done/status/reset                Standalone docs queue
#   version / help                                  Show version / usage

set -e

PROJECT_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
FEATURES_DIR="$PROJECT_ROOT/.spec/features"
CONFIG_FILE="$PROJECT_ROOT/.spec/config.yaml"

# --- helpers ---

VERSION="1.6.0"
EXPLICIT_FEATURE=""

die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo "→ $*"; }
warn() { echo "⚠ $*" >&2; }

iso_now() {
  date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date +"%Y-%m-%dT%H:%M:%SZ"
}

iso_now_compact() {
  date -u +"%Y-%m-%dT%H-%M-%SZ" 2>/dev/null || date +"%Y-%m-%dT%H-%M-%SZ"
}

# Escape a string for safe embedding in JSON values (RFC 8259)
json_escape() {
  printf '%s' "$1" | awk '
    BEGIN { ORS="" }
    {
      gsub(/\\/, "\\\\")
      gsub(/"/, "\\\"")
      gsub(/\t/, "\\t")
      gsub(/\r/, "\\r")
      if (NR > 1) printf "\\n"
      printf "%s", $0
    }'
}

# Read a value from .spec/config.yaml (simple grep-based, no YAML parser)
# Usage: read_config <key> [default]
# Returns the value or default (empty string if no default)
read_config() {
  local key="$1" default="${2:-}"
  if [ -f "$CONFIG_FILE" ]; then
    local val
    val="$(grep "^${key}:" "$CONFIG_FILE" 2>/dev/null | head -1 | sed "s/^${key}:[[:space:]]*//" | sed 's/[[:space:]]*$//')"
    if [ -n "$val" ]; then
      printf '%s' "$val"
      return
    fi
  fi
  printf '%s' "$default"
}

# --- status box rendering ---

# printf "%-Ns" pads by bytes, so multibyte glyphs (em dash, arrows, ✓/●/○) and
# long values break the right border. These helpers pad/truncate by display
# width (UTF-8 character count) to a fixed width so every row aligns.
STATUS_BOX_WIDTH=45

# display_width <string> — UTF-8 character count (each glyph treated as 1 column).
# Counts non-continuation bytes (continuation bytes are 0x80–0xBF).
display_width() {
  LC_ALL=C printf '%s' "$1" | LC_ALL=C tr -d '\200-\277' | LC_ALL=C wc -c | tr -d ' '
}

# box_line <content> — pad/truncate to STATUS_BOX_WIDTH columns, wrap in borders.
# <content> includes its own leading space.
box_line() {
  local s="$1" w="$STATUS_BOX_WIDTH" dw pad
  dw="$(display_width "$s")"
  while [ "$dw" -gt "$w" ]; do
    s="${s%?}"
    dw="$(display_width "$s")"
  done
  pad=$((w - dw))
  printf '│%s%*s│\n' "$s" "$pad" ""
}

# box_rule <left> <right> — horizontal border of STATUS_BOX_WIDTH '─' cells.
box_rule() {
  local i=0 bar=""
  while [ "$i" -lt "$STATUS_BOX_WIDTH" ]; do
    bar="${bar}─"
    i=$((i + 1))
  done
  printf '%s%s%s\n' "$1" "$bar" "$2"
}

# --- per-feature state ---

# Current feature paths (set by set_feature_context / resolve_feature)
FEATURE_DIR=""
STATE_FILE=""
KV_FILE=""
REVISIONS_DIR=""
APPROVED_DIR=""
TASKS_DIR=""

set_feature_context() {
  # set_feature_context <feature-name> — sets global paths for the feature
  FEATURE_DIR="$FEATURES_DIR/$1"
  KV_FILE="$FEATURE_DIR/pipeline.kv"
  STATE_FILE="$FEATURE_DIR/pipeline.json"
  REVISIONS_DIR="$FEATURE_DIR/revisions"
  APPROVED_DIR="$FEATURE_DIR/approved"
  TASKS_DIR="$FEATURE_DIR/tasks"
}

ensure_feature_dir() {
  # ensure_feature_dir <feature-name> — creates feature directory structure
  local fdir="$FEATURES_DIR/$1"
  mkdir -p "$fdir" "$fdir/revisions" "$fdir/approved" "$fdir/tasks"
}

# Read value for KEY from the KV store. Returns 1 if KEY/file is absent.
# Under `set -e`, callers reading OPTIONAL fields MUST guard with `|| echo ""`.
read_field() {
  [ -f "$KV_FILE" ] || return 1
  local _line
  _line="$(grep "^$1=" "$KV_FILE" 2>/dev/null | head -1)" || return 1
  [ -n "$_line" ] || return 1
  printf '%s' "$_line" | cut -d'=' -f2-
}

validate_kv() {
  # Verify required fields exist in KV store; die with diagnostic on failure
  [ -f "$KV_FILE" ] || die "Pipeline state file missing: $KV_FILE"
  local missing=""
  for field in feature phase created_at current_artifact history_count; do
    grep -q "^${field}=" "$KV_FILE" 2>/dev/null || missing="$missing $field"
  done
  if [ -n "$missing" ]; then
    die "Corrupted pipeline state ($KV_FILE): missing fields:$missing. Fix the file manually or remove and re-init."
  fi
  # Verify every line matches key=value format (key: lowercase + digits + underscore)
  local line_num=0
  while IFS= read -r line || [ -n "$line" ]; do
    line_num=$((line_num + 1))
    case "$line" in
      "") continue ;;  # skip blank lines
      [a-z_]*=*) ;;    # valid key=value
      *) die "Corrupted pipeline state ($KV_FILE): invalid line $line_num: $line" ;;
    esac
  done < "$KV_FILE"
}

# Escape a value for safe use in sed replacement string
kv_escape_sed() {
  printf '%s' "$1" | sed -e 's/[&\\/|]/\\&/g'
}

# Validate that a value is safe for the KV store (no =, |, or newlines)
kv_validate_value() {
  case "$1" in
    *'='*) die "KV value must not contain '=': $1" ;;
    *'|'*) die "KV value must not contain '|': $1" ;;
  esac
  # Check for newlines by comparing line count (portable across POSIX shells)
  local line_count
  line_count="$(printf '%s' "$1" | wc -l)"
  if [ "$line_count" -ne 0 ]; then
    die "KV value must not contain newlines: $1"
  fi
}

# Validate artifact path: reject directory traversal and control characters
validate_artifact_path() {
  case "$1" in
    */../*|*/..) die "Artifact path must not contain '..' traversal" ;;
    ../*|..)     die "Artifact path must not contain '..' traversal" ;;
  esac
  if printf '%s' "$1" | grep -q '[[:cntrl:]]' 2>/dev/null; then
    die "Artifact path must not contain control characters"
  fi
}

# Lightweight, non-fatal per-phase content lint (warns only; keywords stay English)
lint_artifact_content() {
  # $1 = phase, $2 = artifact path
  [ -f "$2" ] || return 0
  case "$1" in
    requirements)
      grep -q 'WHEN\|SHALL' "$2" 2>/dev/null || \
        warn "Requirements artifact should contain WHEN/SHALL keywords."
      ;;
    design)
      grep -q 'Correctness\|Property' "$2" 2>/dev/null || \
        warn "Design artifact should contain Correctness Properties."
      ;;
    task-plan)
      grep -q 'T-[0-9]' "$2" 2>/dev/null || \
        warn "Task-plan artifact should contain task IDs (T-1, T-2, ...)."
      grep -q 'RED\|GREEN\|GATE' "$2" 2>/dev/null || \
        warn "Task-plan artifact should contain TDD task types (RED/GREEN/GATE)."
      ;;
    review)
      grep -q 'PASS\|NEEDS_CHANGES\|BLOCK' "$2" 2>/dev/null || \
        warn "Review artifact should contain a verdict (PASS/NEEDS_CHANGES/BLOCK)."
      ;;
  esac
}

write_field() {
  kv_validate_value "$2"
  if [ -f "$KV_FILE" ] && grep -q "^$1=" "$KV_FILE" 2>/dev/null; then
    local tmp="$KV_FILE.tmp"
    local escaped
    escaped="$(kv_escape_sed "$2")"
    sed "s|^$1=.*|$1=$escaped|" "$KV_FILE" > "$tmp" && mv "$tmp" "$KV_FILE"
  else
    echo "$1=$2" >> "$KV_FILE"
  fi
}

detect_active_feature() {
  # Scan all features for one with phase != done.
  # Returns: 0 + name (exactly one), 1 (none), 2 (multiple; lists them on stderr).
  [ -d "$FEATURES_DIR" ] || return 1
  local active=""
  local count=0
  for kv in "$FEATURES_DIR"/*/pipeline.kv; do
    [ -f "$kv" ] || continue
    local phase
    phase="$(grep "^phase=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
    if [ -n "$phase" ] && [ "$phase" != "done" ]; then
      local fname
      fname="$(grep "^feature=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
      active="$fname"
      count=$((count + 1))
    fi
  done
  if [ "$count" -gt 1 ]; then
    warn "Multiple active pipelines found:"
    for kv in "$FEATURES_DIR"/*/pipeline.kv; do
      [ -f "$kv" ] || continue
      local phase fname
      phase="$(grep "^phase=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
      fname="$(grep "^feature=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
      if [ -n "$phase" ] && [ "$phase" != "done" ]; then
        echo "  - $fname (phase: $phase)" >&2
      fi
    done
    return 2
  fi
  if [ -n "$active" ]; then
    echo "$active"
    return 0
  fi
  return 1
}

resolve_feature() {
  if [ -n "$EXPLICIT_FEATURE" ]; then
    # Validate that the explicitly specified feature exists
    if [ ! -f "$FEATURES_DIR/$EXPLICIT_FEATURE/pipeline.kv" ]; then
      die "Feature '$EXPLICIT_FEATURE' not found. Run 'pipeline.sh history' to list features."
    fi
    set_feature_context "$EXPLICIT_FEATURE"
    validate_kv
    return 0
  fi
  local feat rc=0
  feat="$(detect_active_feature)" || rc=$?
  if [ "$rc" -eq 2 ]; then
    die "Multiple active pipelines. Use --feature <name> to select one."
  fi
  if [ -z "$feat" ]; then
    return 1
  fi
  set_feature_context "$feat"
  validate_kv
  return 0
}

next_phase() {
  case "$1" in
    explore)        echo "requirements" ;;
    requirements)   echo "design" ;;
    design)         echo "task-plan" ;;
    task-plan)      echo "implementation" ;;
    implementation) echo "review" ;;
    review)         echo "done" ;;
    done)           echo "" ;;
    *)              echo "" ;;
  esac
}

phase_number() {
  case "$1" in
    explore)        echo "1" ;;
    requirements)   echo "2" ;;
    design)         echo "3" ;;
    task-plan)      echo "4" ;;
    implementation) echo "5" ;;
    review)         echo "6" ;;
    done)           echo "✓" ;;
    *)              echo "?" ;;
  esac
}

# Numeric ordering for comparisons (phase_number is for display)
phase_order() {
  case "$1" in
    explore)        echo 1 ;;
    requirements)   echo 2 ;;
    design)         echo 3 ;;
    task-plan)      echo 4 ;;
    implementation) echo 5 ;;
    review)         echo 6 ;;
    done)           echo 7 ;;
    *)              echo 0 ;;
  esac
}

# Rebuild the JSON file from KV store (for agents to read)
# Uses atomic write (tmp + mv) to prevent corruption on interruption
rebuild_json() {
  validate_kv
  local feature phase created artifact
  feature="$(json_escape "$(read_field feature)")"
  phase="$(read_field phase)"
  created="$(read_field created_at)"
  artifact="$(read_field current_artifact)"
  local history_count
  history_count="$(read_field history_count)"
  [ -z "$history_count" ] && history_count=0

  local tmp_file="$STATE_FILE.tmp"
  {
    printf '{\n'
    printf '  "feature": "%s",\n' "$feature"
    printf '  "phase": "%s",\n' "$phase"
    printf '  "created_at": "%s",\n' "$created"
    if [ -n "$artifact" ]; then
      printf '  "current_artifact": "%s",\n' "$(json_escape "$artifact")"
    else
      printf '  "current_artifact": null,\n'
    fi
    printf '  "history": [\n'

    local i=0
    while [ "$i" -lt "$history_count" ]; do
      local h_phase h_artifact h_approved
      h_phase="$(read_field "history_${i}_phase")"
      h_artifact="$(json_escape "$(read_field "history_${i}_artifact")")"
      h_approved="$(read_field "history_${i}_approved_at")"
      [ "$i" -gt 0 ] && printf ',\n'
      printf '    {"phase": "%s", "artifact": "%s", "approved_at": "%s"}' \
        "$h_phase" "$h_artifact" "$h_approved"
      i=$((i + 1))
    done

    printf '\n  ],\n'

    # Include review_base_commit if set
    local rbc
    rbc="$(read_field review_base_commit 2>/dev/null || echo "")"
    if [ -n "$rbc" ]; then
      printf '  "review_base_commit": "%s",\n' "$(json_escape "$rbc")"
    else
      printf '  "review_base_commit": null,\n'
    fi

    # Include tasks array if a task set is registered
    local task_order_val
    task_order_val="$(read_field task_order 2>/dev/null || echo "")"
    if [ -n "$task_order_val" ]; then
      printf '  "tasks": [\n'
      local _tid _tst _tfirst=1
      for _tid in $task_order_val; do
        _tst="$(read_field "task_status_$_tid" 2>/dev/null || echo "pending")"
        [ -z "$_tst" ] && _tst="pending"
        [ "$_tfirst" -eq 0 ] && printf ',\n'
        _tfirst=0
        printf '    {"id": "%s", "status": "%s", "evidence": "%s"}' \
          "$(json_escape "$_tid")" \
          "$(json_escape "$_tst")" \
          "$(json_escape "$TASKS_DIR/$_tid.md")"
      done
      printf '\n  ],\n'
    else
      printf '  "tasks": [],\n'
    fi

    # Include finish fields if set (finish_action is set by 'abandon')
    local fa ft
    fa="$(read_field finish_action 2>/dev/null || echo "")"
    ft="$(read_field finished_at 2>/dev/null || echo "")"
    if [ -n "$fa" ]; then
      printf '  "finish_action": "%s",\n' "$(json_escape "$fa")"
      printf '  "finished_at": "%s"\n' "$(json_escape "$ft")"
    else
      printf '  "finish_action": null,\n'
      printf '  "finished_at": null\n'
    fi

    printf '}\n'
  } > "$tmp_file"
  mv -f "$tmp_file" "$STATE_FILE"
}

# --- commands ---

cmd_init() {
  local feature=""
  while [ $# -gt 0 ]; do
    case "$1" in
      -*) die "Unknown flag for init: $1" ;;
      *)
        [ -n "$feature" ] && die "Unexpected argument: $1"
        feature="$1"; shift
        ;;
    esac
  done

  [ -z "$feature" ] && die "Usage: pipeline.sh init <feature-name>"

  # Validate feature name (kebab-case)
  case "$feature" in
    *[!a-z0-9-]*) die "Feature name must be kebab-case (e.g. grpc-streaming-support)" ;;
    -*|*-)        die "Feature name must be kebab-case (e.g. grpc-streaming-support)" ;;
    *--*)         die "Feature name must be kebab-case (e.g. grpc-streaming-support)" ;;
    [!a-z]*)      die "Feature name must be kebab-case (e.g. grpc-streaming-support)" ;;
  esac

  if [ ${#feature} -gt 64 ]; then
    die "Feature name too long (max 64 chars): $feature"
  fi

  local fdir="$FEATURES_DIR/$feature"

  # Check if feature already exists
  if [ -f "$fdir/pipeline.kv" ]; then
    local existing_phase
    existing_phase="$(grep "^phase=" "$fdir/pipeline.kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
    if [ "$existing_phase" = "done" ]; then
      die "Feature '$feature' already completed. Choose a different name."
    else
      warn "Active pipeline for '$feature' exists (phase: $existing_phase)"
      die "Complete or choose a different feature name."
    fi
  fi

  ensure_feature_dir "$feature"
  set_feature_context "$feature"

  # Initialize KV store (atomic write: tmp + mv)
  local kv_tmp="$KV_FILE.tmp"
  {
    echo "feature=$feature"
    echo "phase=explore"
    echo "created_at=$(iso_now)"
    echo "current_artifact="
    echo "history_count=0"
  } > "$kv_tmp"
  mv -f "$kv_tmp" "$KV_FILE"

  rebuild_json
  info "Pipeline initialized for '$feature'"
  info "Phase: [1/6] explore"
  info "Artifacts: .spec/features/$feature/"
  info "Read template: ./templates/explore.md"
}

cmd_status() {
  if ! resolve_feature; then
    info "No active pipeline."
    # Show completed features if any
    if [ -d "$FEATURES_DIR" ]; then
      local has_completed=0
      for kv in "$FEATURES_DIR"/*/pipeline.kv; do
        [ -f "$kv" ] || continue
        local phase
        phase="$(grep "^phase=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
        if [ "$phase" = "done" ]; then
          if [ "$has_completed" -eq 0 ]; then
            echo ""
            echo "Completed features:"
            has_completed=1
          fi
          local fname
          fname="$(grep "^feature=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
          printf "  ✓ %s\n" "$fname"
        fi
      done
    fi
    echo ""
    info "Run: pipeline.sh init <feature-name>"
    return 0
  fi

  local feature phase artifact history_count
  feature="$(read_field feature)"
  phase="$(read_field phase)"
  artifact="$(read_field current_artifact)"
  history_count="$(read_field history_count)"
  [ -z "$history_count" ] && history_count=0

  echo ""
  box_rule "┌" "┐"
  box_line " Feature: $feature"
  box_line " Phase:   [$(phase_number "$phase")/6] $phase"
  if [ -n "$artifact" ]; then
    box_line " Artifact: ${artifact##*/}"
  else
    box_line " Artifact: (none, register before approve)"
  fi
  # Show task progress during the implementation phase
  if [ "$phase" = "implementation" ]; then
    local _order
    _order="$(read_field task_order 2>/dev/null || echo "")"
    if [ -n "$_order" ]; then
      local _tid _tst _ttot=0 _tdone=0
      for _tid in $_order; do
        _tst="$(read_field "task_status_$_tid" 2>/dev/null || echo "pending")"
        _ttot=$((_ttot + 1))
        [ "$_tst" = "done" ] && _tdone=$((_tdone + 1))
      done
      box_line " Tasks: $_tdone/$_ttot done"
    fi
  fi
  box_rule "├" "┤"

  # Show pipeline progress
  local e_mark="○" r_mark="○" d_mark="○" t_mark="○" i_mark="○" rev_mark="○"
  case "$phase" in
    explore)        e_mark="●" ;;
    requirements)   e_mark="✓"; r_mark="●" ;;
    design)         e_mark="✓"; r_mark="✓"; d_mark="●" ;;
    task-plan)      e_mark="✓"; r_mark="✓"; d_mark="✓"; t_mark="●" ;;
    implementation) e_mark="✓"; r_mark="✓"; d_mark="✓"; t_mark="✓"; i_mark="●" ;;
    review)         e_mark="✓"; r_mark="✓"; d_mark="✓"; t_mark="✓"; i_mark="✓"; rev_mark="●" ;;
    done)           e_mark="✓"; r_mark="✓"; d_mark="✓"; t_mark="✓"; i_mark="✓"; rev_mark="✓" ;;
  esac
  box_line " $e_mark Ex → $r_mark Rq → $d_mark Ds → $t_mark Tp → $i_mark Im → $rev_mark Rv"
  box_rule "└" "┘"

  # Show history
  if [ "$history_count" -gt 0 ]; then
    echo ""
    echo "Completed phases:"
    local i=0
    while [ "$i" -lt "$history_count" ]; do
      local h_phase h_artifact h_approved
      h_phase="$(read_field "history_${i}_phase")"
      h_artifact="$(read_field "history_${i}_artifact")"
      h_approved="$(read_field "history_${i}_approved_at")"
      printf "  [%s] %-15s → %s (approved: %s)\n" "$((i+1))" "$h_phase" "$h_artifact" "$h_approved"
      i=$((i + 1))
    done
  fi

  # Hint for next action
  echo ""
  if [ "$phase" = "done" ]; then
    info "Pipeline complete."
  elif [ -z "$artifact" ]; then
    info "Next: register artifact with 'pipeline.sh artifact <path>'"
    info "Then: 'pipeline.sh approve' after user approval"
  else
    info "Artifact registered. Ask user to approve, then run 'pipeline.sh approve'"
  fi
  echo ""
}

cmd_artifact() {
  resolve_feature || die "No active pipeline. Run 'pipeline.sh init <feature>' first."

  local phase
  phase="$(read_field phase)"
  [ "$phase" = "done" ] && die "Pipeline is complete. Nothing to register."

  local path="$1"

  # If no path given, use the default: .spec/features/<feature>/<phase>.md
  if [ -z "$path" ]; then
    path="$FEATURE_DIR/${phase}.md"
  fi

  validate_artifact_path "$path"
  [ -f "$path" ] || die "Artifact file does not exist: $path"

  lint_artifact_content "$phase" "$path"

  # Save a snapshot of the artifact being registered (revision tracking)
  local rev_count
  rev_count="$(read_field "revision_count_${phase}" || echo "")"
  [ -z "$rev_count" ] && rev_count=0
  rev_count=$((rev_count + 1))
  local rev_name
  rev_name="${phase}-rev-${rev_count}-$(iso_now_compact).md"
  cp "$path" "$REVISIONS_DIR/$rev_name"
  write_field "revision_count_${phase}" "$rev_count"
  if [ "$rev_count" -gt 1 ]; then
    info "Revision $rev_count saved: $rev_name"
  fi

  write_field current_artifact "$path"
  rebuild_json
  info "Artifact registered for phase '$phase': $path"
}

cmd_approve() {
  resolve_feature || die "No active pipeline."

  local phase artifact history_count
  phase="$(read_field phase)"
  artifact="$(read_field current_artifact)"
  history_count="$(read_field history_count)"
  [ -z "$history_count" ] && history_count=0

  [ "$phase" = "done" ] && die "Pipeline already complete."
  [ -z "$artifact" ] && die "No artifact registered for phase '$phase'. Run 'pipeline.sh artifact <path>' first."
  [ -f "$artifact" ] || die "Artifact file no longer exists: $artifact. Re-register with 'pipeline.sh artifact <path>'."

  # Snapshot artifact contents
  cp "$artifact" "$APPROVED_DIR/${phase}.md"

  # Record base commit for review phase (git diff source)
  if [ "$phase" = "task-plan" ]; then
    local base_commit
    base_commit="$(git rev-parse HEAD 2>/dev/null || echo "")"
    write_field review_base_commit "$base_commit"
  fi

  # Record in history
  write_field "history_${history_count}_phase" "$phase"
  write_field "history_${history_count}_artifact" "$artifact"
  write_field "history_${history_count}_approved_at" "$(iso_now)"
  history_count=$((history_count + 1))
  write_field history_count "$history_count"

  # Advance phase
  local next
  next="$(next_phase "$phase")"
  write_field phase "$next"
  write_field current_artifact ""

  rebuild_json

  if [ "$next" = "done" ]; then
    echo ""
    echo "✅ Pipeline complete!"
    echo ""
    echo "All artifacts:"
    local i=0
    while [ "$i" -lt "$history_count" ]; do
      printf "  [%s] %s → %s\n" "$((i+1))" \
        "$(read_field "history_${i}_phase")" \
        "$(read_field "history_${i}_artifact")"
      i=$((i + 1))
    done
    echo ""
    local feat
    feat="$(read_field feature)"
    info "Artifacts saved in: .spec/features/$feat/"
    info "Next: check documentation with 'pipeline.sh docs-check'."
  else
    info "Phase '$phase' approved."
    info "Advanced to: [$(phase_number "$next")/6] $next"
    info "Read template: ./templates/${next}.md"
  fi
}

cmd_task() {
  local task_id="${1:-}"
  local new_status="${2:-done}"
  [ -z "$task_id" ] && die "Usage: pipeline.sh task <T-N> [done|wip|blocked]"

  case "$new_status" in
    done|wip|blocked|pending) ;;
    *) die "Invalid status '$new_status'. Use: done | wip | blocked | pending." ;;
  esac

  resolve_feature || die "No active pipeline."

  local phase
  phase="$(read_field phase)"
  [ "$phase" = "implementation" ] || die "Task tracking is only available during the implementation phase (current: $phase)."

  local order
  order="$(read_field task_order 2>/dev/null || echo "")"
  [ -z "$order" ] && die "No task set. Run 'pipeline.sh task-init <T-1> <T-2> ...' first (IDs from the approved task plan)."

  case " $order " in
    *" $task_id "*) ;;
    *) die "Task '$task_id' is not in the task set ($order). Run 'pipeline.sh tasks' to list." ;;
  esac

  write_field "task_status_$task_id" "$new_status"
  rebuild_json
  info "Task $task_id → $new_status"
}

cmd_task_init() {
  resolve_feature || die "No active pipeline. Run 'pipeline.sh init <feature>' first."

  local phase
  phase="$(read_field phase)"
  [ "$phase" = "implementation" ] || die "task-init is only available during the implementation phase (current: $phase)."

  [ $# -gt 0 ] || die "Usage: pipeline.sh task-init <T-1> <T-2> ... (task IDs from the approved task plan)"

  local existing
  existing="$(read_field task_order 2>/dev/null || echo "")"
  [ -n "$existing" ] && die "Task set already exists ($existing). Run 'pipeline.sh task-reset' first to re-initialize."

  # Validate and de-duplicate the provided task IDs
  local order="" id
  for id in "$@"; do
    case "$id" in
      *[!A-Za-z0-9._-]*) die "Invalid task ID '$id' (allowed: letters, digits, '.', '_', '-')." ;;
      [!A-Za-z]*)        die "Task ID '$id' must start with a letter (e.g., T-1)." ;;
    esac
    case " $order " in
      *" $id "*) warn "Duplicate task ID '$id' ignored."; continue ;;
    esac
    order="$order $id"
  done
  order="${order# }"
  [ -n "$order" ] || die "No valid task IDs provided."

  write_field task_order "$order"

  mkdir -p "$TASKS_DIR"
  local count=0
  for id in $order; do
    write_field "task_status_$id" "pending"
    if [ ! -f "$TASKS_DIR/$id.md" ]; then
      {
        echo "# $id"
        echo ""
        echo "<!-- Evidence for this task: code changes, test stdout, subagent report."
        echo "     Status is tracked by the pipeline — see 'pipeline.sh tasks'. -->"
        echo ""
        echo "## Changes"
        echo ""
        echo "## Verification"
        echo ""
        echo "## Notes"
      } > "$TASKS_DIR/$id.md"
    fi
    count=$((count + 1))
  done

  rebuild_json
  info "Task set initialized: $count task(s) — $order"
  info "Evidence stubs created in: $TASKS_DIR/"
  info "Mark progress with: pipeline.sh task <T-N> [done|wip|blocked]"
}

cmd_task_next() {
  resolve_feature || die "No active pipeline."

  local order
  order="$(read_field task_order 2>/dev/null || echo "")"
  [ -z "$order" ] && die "No task set. Run 'pipeline.sh task-init <T-1> <T-2> ...' first."

  local id st
  for id in $order; do
    st="$(read_field "task_status_$id" 2>/dev/null || echo "pending")"
    if [ "$st" = "pending" ] || [ "$st" = "wip" ]; then
      printf '%s\t%s\n' "$id" "$TASKS_DIR/$id.md"
      return 0
    fi
  done

  local blocked="" b_id b_st
  for b_id in $order; do
    b_st="$(read_field "task_status_$b_id" 2>/dev/null || echo "")"
    [ "$b_st" = "blocked" ] && blocked="$blocked $b_id"
  done
  if [ -n "$blocked" ]; then
    info "No actionable tasks. Blocked:$blocked. Resolve blockers, then 'pipeline.sh task <T-N> wip'."
  else
    info "All tasks done. Register the implementation report with 'pipeline.sh artifact'."
  fi
  return 0
}

cmd_tasks() {
  resolve_feature || die "No active pipeline."

  local order
  order="$(read_field task_order 2>/dev/null || echo "")"
  if [ -z "$order" ]; then
    info "No task set. Run 'pipeline.sh task-init <T-1> <T-2> ...' first."
    return 0
  fi

  local id st mark total=0 done_count=0
  echo ""
  echo "Tasks (status in pipeline.kv; evidence in $TASKS_DIR/):"
  for id in $order; do
    st="$(read_field "task_status_$id" 2>/dev/null || echo "pending")"
    [ -z "$st" ] && st="pending"
    total=$((total + 1))
    case "$st" in
      done)    mark="✓"; done_count=$((done_count + 1)) ;;
      wip)     mark="◐" ;;
      blocked) mark="✗" ;;
      *)       mark="○" ;;
    esac
    printf "  %s %-12s %s\n" "$mark" "$id" "$st"
  done
  echo ""
  info "$done_count/$total done"
}

cmd_task_reset() {
  resolve_feature || die "No active pipeline."

  local phase
  phase="$(read_field phase)"
  [ "$phase" = "implementation" ] || die "task-reset is only available during the implementation phase (current: $phase)."

  local order
  order="$(read_field task_order 2>/dev/null || echo "")"
  if [ -z "$order" ]; then
    info "No task set to reset."
    return 0
  fi

  local id
  for id in $order; do
    write_field "task_status_$id" ""
  done
  write_field task_order ""
  rebuild_json
  info "Task set cleared (evidence files kept in $TASKS_DIR/)."
  info "Re-initialize with: pipeline.sh task-init <T-1> <T-2> ..."
}

cmd_history() {
  if [ ! -d "$FEATURES_DIR" ]; then
    info "No features found."
    return 0
  fi

  local found=0
  for kv in "$FEATURES_DIR"/*/pipeline.kv; do
    [ -f "$kv" ] || continue
    found=1
    local fname phase created
    fname="$(grep "^feature=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
    phase="$(grep "^phase=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
    created="$(grep "^created_at=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"

    local status_icon
    if [ "$phase" = "done" ]; then
      status_icon="✓"
    else
      status_icon="●"
    fi

    printf "  %s %-25s [%s/6] %-15s (created: %s)\n" \
      "$status_icon" "$fname" "$(phase_number "$phase")" "$phase" "$created"
  done

  if [ "$found" -eq 0 ]; then
    info "No features found."
  fi
}

cmd_revisions() {
  resolve_feature || die "No active pipeline."

  local phase
  phase="$(read_field phase)"

  local target_phase="${1:-$phase}"
  # Validate target phase
  case "$target_phase" in
    explore|requirements|design|task-plan|implementation|review|all) ;;
    *) die "Unknown phase: $target_phase. Use: explore, requirements, design, task-plan, implementation, review, or all." ;;
  esac

  local found=0
  local tmp
  tmp="$(mktemp)"
  if [ "$target_phase" = "all" ]; then
    echo "All revisions:"
    for p in explore requirements design task-plan implementation review; do
      find "$REVISIONS_DIR" -name "${p}-rev-*" 2>/dev/null | sort > "$tmp"
      while IFS= read -r f; do
        printf "  [%s] %s\n" "$p" "$(basename "$f")"
        found=1
      done < "$tmp"
    done
  else
    echo "Revisions for phase '$target_phase':"
    find "$REVISIONS_DIR" -name "${target_phase}-rev-*" 2>/dev/null | sort > "$tmp"
    while IFS= read -r f; do
      printf "  %s\n" "$(basename "$f")"
      found=1
    done < "$tmp"
  fi
  rm -f "$tmp"

  if [ "$found" -eq 0 ]; then
    info "No revisions recorded yet."
  fi
}

cmd_version() {
  echo "Spec-Driven Dev Pipeline v${VERSION}"
}

# check_file_staleness <file> <templates_dir> <freshness_days> <now_epoch>
# Outputs tab-separated: generated template age_days stale scope_changed
check_file_staleness() {
  local f="$1" templates_dir="$2" freshness_days="$3" now_epoch="$4"
  local generated="null" template="null" age_days="null" stale="false" scope_changed="null"

  local first_line
  first_line="$(head -1 "$f" 2>/dev/null)"
  case "$first_line" in
    *"<!-- generated:"*"template:"*"-->"*)
      local gen_date gen_tmpl
      gen_date="$(echo "$first_line" | sed 's/.*<!-- generated: \([0-9-]*\),.*/\1/')"
      gen_tmpl="$(echo "$first_line" | sed 's/.*template: \([^ ]*\) -->.*/\1/')"
      if [ -n "$gen_date" ]; then
        generated="\"$gen_date\""
        template="\"$gen_tmpl\""
        local gen_epoch
        gen_epoch="$(date -j -f '%Y-%m-%d' "$gen_date" '+%s' 2>/dev/null || date -d "$gen_date" '+%s' 2>/dev/null || echo 0)"
        if [ "$gen_epoch" -gt 0 ] && [ "$now_epoch" -gt 0 ]; then
          age_days=$(( (now_epoch - gen_epoch) / 86400 ))

          local tmpl_file="$templates_dir/$gen_tmpl"
          if [ -f "$tmpl_file" ]; then
            local scope_line
            scope_line="$(head -1 "$tmpl_file" 2>/dev/null)"
            case "$scope_line" in
              "<!-- scope:"*"-->")
                local patterns
                patterns="$(echo "$scope_line" | sed 's/<!-- scope: //' | sed 's/ -->//' | sed 's/,[[:space:]]*/\n/g' | tr '\n' ' ' | sed 's/[[:space:]]*$//')"
                if [ -n "$patterns" ]; then
                  local git_hits
                  # shellcheck disable=SC2086
                  git_hits="$(cd "$PROJECT_ROOT" && set -f && git log --oneline --since="$gen_date" -- $patterns 2>/dev/null | head -1)"
                  if [ -n "$git_hits" ]; then
                    scope_changed="true"
                    if [ "$age_days" -gt "$freshness_days" ]; then
                      stale="true"
                    fi
                  else
                    scope_changed="false"
                  fi
                fi
                ;;
              *)
                if [ "$age_days" -gt "$freshness_days" ]; then
                  stale="true"
                fi
                ;;
            esac
          else
            if [ "$age_days" -gt "$freshness_days" ]; then
              stale="true"
            fi
          fi
        fi
      fi
      ;;
  esac
  printf '%s\t%s\t%s\t%s\t%s' "$generated" "$template" "$age_days" "$stale" "$scope_changed"
}

cmd_docs_check() {
  local config_file="$CONFIG_FILE"
  local docs_dir=".spec"
  local freshness_days=30

  # Read docs_dir and doc_freshness_days from config.yaml if it exists
  if [ -f "$config_file" ]; then
    local configured_dir
    configured_dir="$(grep '^docs_dir:' "$config_file" 2>/dev/null | head -1 | sed 's/^docs_dir:[[:space:]]*//' | sed 's/[[:space:]]*$//')"
    if [ -n "$configured_dir" ]; then
      docs_dir="$configured_dir"
    fi
    local configured_days
    configured_days="$(grep '^doc_freshness_days:' "$config_file" 2>/dev/null | head -1 | sed 's/^doc_freshness_days:[[:space:]]*//' | sed 's/[[:space:]]*$//')"
    if [ -n "$configured_days" ]; then
      freshness_days="$configured_days"
    fi
  fi

  local full_path="$PROJECT_ROOT/$docs_dir"
  local templates_dir="$SKILL_DIR/templates/docs"
  local now_epoch
  now_epoch="$(date +%s 2>/dev/null || echo 0)"

  if [ -d "$full_path" ]; then
    printf '{"exists": true, "dir": "%s", "freshness_days": %d, "files": [' "$(json_escape "$docs_dir")" "$freshness_days"

    # Single scan: find files into tmpfile, iterate once, capture stale names
    local tmp_files tmp_stale
    tmp_files="$(mktemp)"
    tmp_stale="$(mktemp)"
    find "$full_path" -maxdepth 1 -type f -name '*.md' 2>/dev/null | sort > "$tmp_files"

    local first=1
    while IFS= read -r f; do
      local fname result generated template age_days stale scope_changed
      fname="$(basename "$f")"
      result="$(check_file_staleness "$f" "$templates_dir" "$freshness_days" "$now_epoch")"

      generated="$(printf '%s' "$result" | cut -f1)"
      template="$(printf '%s' "$result" | cut -f2)"
      age_days="$(printf '%s' "$result" | cut -f3)"
      stale="$(printf '%s' "$result" | cut -f4)"
      scope_changed="$(printf '%s' "$result" | cut -f5)"

      if [ "$first" -eq 1 ]; then
        first=0
      else
        printf ', '
      fi
      printf '{"name": "%s", "generated": %s, "template": %s, "age_days": %s, "stale": %s, "scope_changed": %s}' \
        "$(json_escape "$fname")" "$generated" "$template" "$age_days" "$stale" "$scope_changed"

      if [ "$stale" = "true" ]; then
        echo "$fname" >> "$tmp_stale"
      fi
    done < "$tmp_files"

    printf '], "stale": ['
    # Read stale names from tmpfile (no re-scan needed)
    local sfirst=1
    if [ -s "$tmp_stale" ]; then
      while IFS= read -r sname; do
        if [ "$sfirst" -eq 1 ]; then
          sfirst=0
        else
          printf ', '
        fi
        printf '"%s"' "$(json_escape "$sname")"
      done < "$tmp_stale"
    fi
    printf ']}\n'

    rm -f "$tmp_files" "$tmp_stale"
  else
    printf '{"exists": false, "dir": "%s", "freshness_days": %d, "files": [], "stale": []}\n' "$(json_escape "$docs_dir")" "$freshness_days"
  fi
}

# --- standalone docs queue ---

DOCS_QUEUE_FILE="$PROJECT_ROOT/.spec/.docs-queue.kv"

docs_queue_read() {
  # docs_queue_read <key> — read value from queue file.
  # Returns 1 if key/file is absent — guard with `|| echo ""` under `set -e`.
  [ -f "$DOCS_QUEUE_FILE" ] || return 1
  local _line
  _line="$(grep "^$1=" "$DOCS_QUEUE_FILE" 2>/dev/null | head -1)" || return 1
  [ -n "$_line" ] || return 1
  printf '%s' "$_line" | sed "s/^$1=//"
}

docs_queue_write_status() {
  # docs_queue_write_status <index> <status>
  local idx="$1" status="$2"
  local tmp="$DOCS_QUEUE_FILE.tmp"
  grep -v "^template_${idx}_status=" "$DOCS_QUEUE_FILE" > "$tmp" 2>/dev/null || true
  echo "template_${idx}_status=$status" >> "$tmp"
  mv -f "$tmp" "$DOCS_QUEUE_FILE"
}

docs_template_name() {
  # Extract template name from generated file metadata (line 1)
  local f="$1"
  local first_line
  first_line="$(head -1 "$f" 2>/dev/null)"
  case "$first_line" in
    "<!-- generated:"*"-->")
      printf '%s' "$first_line" | sed -n 's/.*template:[[:space:]]*\([^[:space:]]*\)\.md[[:space:]]*-->/\1/p'
      ;;
  esac
}

cmd_docs_init() {
  local mode="all"
  local explicit_templates=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --all)    mode="all"; shift ;;
      --update) mode="update"; shift ;;
      -*)       die "Unknown flag for docs-init: $1" ;;
      *)
        explicit_templates="$explicit_templates $1"
        mode="explicit"
        shift
        ;;
    esac
  done

  [ -f "$DOCS_QUEUE_FILE" ] && die "Docs queue already exists at $DOCS_QUEUE_FILE. Run 'pipeline.sh docs-reset' first."

  local templates_dir="$SKILL_DIR/templates/docs"
  local docs_dir
  docs_dir="$(read_config docs_dir ".spec")"
  local full_path="$PROJECT_ROOT/$docs_dir"

  local templates=""
  case "$mode" in
    all)
      for f in "$templates_dir"/*.md; do
        local name
        name="$(basename "$f" .md)"
        [ "$name" = "README" ] && continue
        templates="$templates $name"
      done
      ;;
    update)
      [ -d "$full_path" ] || die "Docs directory '$docs_dir' does not exist. Use --all to bootstrap."
      local freshness_days
      freshness_days="$(read_config doc_freshness_days "30")"
      local now_epoch
      now_epoch="$(date +%s 2>/dev/null || echo 0)"
      # Iterate stale files, extract template names
      local tmp_list
      tmp_list="$(mktemp)"
      find "$full_path" -maxdepth 1 -type f -name '*.md' 2>/dev/null | sort > "$tmp_list"
      while IFS= read -r f; do
        local result stale tmpl
        result="$(check_file_staleness "$f" "$templates_dir" "$freshness_days" "$now_epoch")"
        stale="$(printf '%s' "$result" | cut -f4)"
        if [ "$stale" = "true" ]; then
          tmpl="$(docs_template_name "$f")"
          if [ -n "$tmpl" ]; then
            case " $templates " in
              *" $tmpl "*) ;;
              *) templates="$templates $tmpl" ;;
            esac
          fi
        fi
      done < "$tmp_list"
      rm -f "$tmp_list"
      ;;
    explicit)
      templates="$explicit_templates"
      ;;
  esac

  mkdir -p "$(dirname "$DOCS_QUEUE_FILE")"

  # Build queue file
  local count=0
  local tmp_queue="$DOCS_QUEUE_FILE.tmp"
  {
    echo "created_at=$(iso_now)"
    echo "docs_dir=$docs_dir"
    echo "mode=$mode"
  } > "$tmp_queue"

  for t in $templates; do
    if [ ! -f "$templates_dir/$t.md" ]; then
      warn "Template not found, skipping: $t"
      continue
    fi
    echo "template_${count}=$t" >> "$tmp_queue"
    echo "template_${count}_status=pending" >> "$tmp_queue"
    count=$((count + 1))
  done
  echo "total=$count" >> "$tmp_queue"

  if [ "$count" -eq 0 ]; then
    rm -f "$tmp_queue"
    info "No templates to queue (nothing stale or none selected)."
    return 0
  fi

  mv -f "$tmp_queue" "$DOCS_QUEUE_FILE"

  info "Docs queue created: $count template(s), mode=$mode."
  info "Next: pipeline.sh docs-next"
  info ""
  info "Execution strategy:"
  info "  - If your toolset supports subagent dispatch (Task/Composer/etc):"
  info "    use SUBAGENT mode — dispatch up to 3 templates in parallel."
  info "  - Otherwise: SEQUENTIAL mode — one template per iteration."
  info "  See ./templates/docs-maintenance.md § Standalone Documentation Workflow."
}

cmd_docs_next() {
  [ -f "$DOCS_QUEUE_FILE" ] || die "No docs queue. Run 'pipeline.sh docs-init' first."

  local total
  total="$(docs_queue_read total || echo "")"
  [ -z "$total" ] && total=0

  local i=0
  local templates_dir="$SKILL_DIR/templates/docs"
  local docs_dir
  docs_dir="$(docs_queue_read docs_dir || echo "")"

  while [ "$i" -lt "$total" ]; do
    local status
    status="$(docs_queue_read "template_${i}_status" || echo "")"
    if [ "$status" = "pending" ]; then
      local name
      name="$(docs_queue_read "template_${i}" || echo "")"
      printf '%s\t%s\n' "$name" "$templates_dir/$name.md"
      # Sequential mode hint after position 3
      if [ "$i" -ge 3 ]; then
        info "" >&2
        info "Tip: if context feels heavy, start a fresh chat and resume" >&2
        info "     with 'pipeline.sh docs-status' (sequential mode)." >&2
      fi
      return 0
    fi
    i=$((i + 1))
  done

  info "Docs queue complete: all $total template(s) processed."
  info "Run 'pipeline.sh docs-reset' to clear the queue."
  return 0
}

cmd_docs_done() {
  local name="${1:-}"
  [ -z "$name" ] && die "Usage: pipeline.sh docs-done <template>"
  [ -f "$DOCS_QUEUE_FILE" ] || die "No docs queue. Run 'pipeline.sh docs-init' first."

  local total
  total="$(docs_queue_read total || echo "")"
  [ -z "$total" ] && total=0

  local i=0
  while [ "$i" -lt "$total" ]; do
    local entry
    entry="$(docs_queue_read "template_${i}" || echo "")"
    if [ "$entry" = "$name" ]; then
      local status
      status="$(docs_queue_read "template_${i}_status" || echo "")"
      if [ "$status" = "done" ]; then
        warn "Template '$name' already marked done."
        return 0
      fi
      docs_queue_write_status "$i" "done"
      info "Marked done: $name ($((i + 1))/$total)"
      # Check if queue is now complete
      local remaining=0
      local j=0
      while [ "$j" -lt "$total" ]; do
        local s
        s="$(docs_queue_read "template_${j}_status" || echo "")"
        [ "$s" = "pending" ] && remaining=$((remaining + 1))
        j=$((j + 1))
      done
      if [ "$remaining" -eq 0 ]; then
        info "Docs queue complete. Run 'pipeline.sh docs-reset' to clear it."
      fi
      return 0
    fi
    i=$((i + 1))
  done

  die "Template '$name' not found in queue."
}

cmd_docs_status() {
  if [ ! -f "$DOCS_QUEUE_FILE" ]; then
    printf '{"exists": false}\n'
    return 0
  fi

  local total docs_dir mode created_at
  total="$(docs_queue_read total || echo "")"
  docs_dir="$(docs_queue_read docs_dir || echo "")"
  mode="$(docs_queue_read mode || echo "")"
  created_at="$(docs_queue_read created_at || echo "")"
  [ -z "$total" ] && total=0

  local completed=0
  local pending_list=""
  local current=""
  local first_pending_set=0
  local i=0
  while [ "$i" -lt "$total" ]; do
    local name status
    name="$(docs_queue_read "template_${i}" || echo "")"
    status="$(docs_queue_read "template_${i}_status" || echo "")"
    if [ "$status" = "done" ]; then
      completed=$((completed + 1))
    else
      if [ "$first_pending_set" -eq 0 ]; then
        current="$name"
        first_pending_set=1
      fi
      if [ -z "$pending_list" ]; then
        pending_list="\"$(json_escape "$name")\""
      else
        pending_list="$pending_list, \"$(json_escape "$name")\""
      fi
    fi
    i=$((i + 1))
  done

  printf '{"exists": true, "total": %d, "completed": %d, "current": %s, "pending": [%s], "mode": "%s", "docs_dir": "%s", "created_at": "%s"}\n' \
    "$total" \
    "$completed" \
    "$([ -n "$current" ] && printf '"%s"' "$(json_escape "$current")" || printf 'null')" \
    "$pending_list" \
    "$(json_escape "$mode")" \
    "$(json_escape "$docs_dir")" \
    "$(json_escape "$created_at")"
}

cmd_docs_reset() {
  if [ ! -f "$DOCS_QUEUE_FILE" ]; then
    info "No docs queue to reset."
    return 0
  fi

  local total completed=0
  total="$(docs_queue_read total || echo "")"
  [ -z "$total" ] && total=0
  local i=0
  while [ "$i" -lt "$total" ]; do
    local s
    s="$(docs_queue_read "template_${i}_status" || echo "")"
    [ "$s" = "done" ] && completed=$((completed + 1))
    i=$((i + 1))
  done

  rm -f "$DOCS_QUEUE_FILE"
  info "Docs queue reset (was: $completed/$total completed)."
}

cmd_config_check() {
  [ -f "$CONFIG_FILE" ] || { info "No config file found: $CONFIG_FILE"; return 0; }

  local valid_keys=" context rules.explore rules.requirements rules.design rules.task-plan rules.implementation rules.review rules.docs test_skill test_reference docs_dir doc_freshness_days "
  local errors=0

  info "Checking $CONFIG_FILE ..."

  # Extract keys and validate against whitelist
  local tmp
  tmp="$(mktemp)"
  grep '^[a-z]' "$CONFIG_FILE" 2>/dev/null > "$tmp" || true
  while IFS= read -r line; do
    local key
    key="$(printf '%s' "$line" | sed 's/:.*//')"
    case "$valid_keys" in
      *" $key "*) ;;
      *) warn "Unknown key: '$key'"; errors=$((errors + 1)) ;;
    esac
  done < "$tmp"
  rm -f "$tmp"

  # Type checks
  local val
  val="$(read_config doc_freshness_days "")"
  if [ -n "$val" ]; then
    case "$val" in
      *[!0-9]*) warn "doc_freshness_days must be numeric, got: '$val'"; errors=$((errors + 1)) ;;
    esac
  fi

  if [ "$errors" -eq 0 ]; then
    info "Config OK — all keys valid."
  else
    warn "$errors problem(s) found."
    return 1
  fi
}

cmd_doctor() {
  local problems=0 warnings=0
  echo "Spec-Driven Dev Pipeline v${VERSION} — environment check"
  echo ""
  info "project root: $PROJECT_ROOT"
  info "skill dir:    $SKILL_DIR"
  echo ""

  # Required external tools
  local tool
  for tool in grep sed awk date mktemp; do
    if command -v "$tool" >/dev/null 2>&1; then
      info "tool: $tool"
    else
      warn "tool: $tool MISSING (required)"
      problems=$((problems + 1))
    fi
  done

  # Phase + docs templates
  local tmpl missing=""
  for tmpl in explore requirements design task-plan implementation review; do
    [ -f "$SKILL_DIR/templates/${tmpl}.md" ] || missing="$missing ${tmpl}.md"
  done
  [ -f "$SKILL_DIR/templates/docs-maintenance.md" ] || missing="$missing docs-maintenance.md"
  if [ -n "$missing" ]; then
    warn "templates: missing in $SKILL_DIR/templates:$missing"
    problems=$((problems + 1))
  else
    info "templates: all present ($SKILL_DIR/templates)"
  fi

  # .spec writability
  local spec_dir="$PROJECT_ROOT/.spec"
  if [ -d "$spec_dir" ]; then
    if [ -w "$spec_dir" ]; then
      info ".spec: writable ($spec_dir)"
    else
      warn ".spec: NOT writable ($spec_dir)"
      problems=$((problems + 1))
    fi
  elif [ -w "$PROJECT_ROOT" ]; then
    info ".spec: absent (will be created under writable project root)"
  else
    warn "project root NOT writable ($PROJECT_ROOT) — cannot create .spec"
    problems=$((problems + 1))
  fi

  # git (read-only features: review base commit, docs staleness, root detection)
  if command -v git >/dev/null 2>&1; then
    if git rev-parse --git-dir >/dev/null 2>&1; then
      info "git: available, inside a repository"
    else
      warn "git: available but not a repository — review base commit and docs staleness limited"
      warnings=$((warnings + 1))
    fi
  else
    warn "git: not found — root falls back to CWD; review base commit and docs staleness disabled"
    warnings=$((warnings + 1))
  fi

  # Config (optional)
  if [ -f "$CONFIG_FILE" ]; then
    info "config: $CONFIG_FILE (run 'pipeline.sh config-check' to validate)"
  else
    info "config: none (optional; defaults apply)"
  fi

  # Active pipelines
  local active=0 kv ph
  if [ -d "$FEATURES_DIR" ]; then
    for kv in "$FEATURES_DIR"/*/pipeline.kv; do
      [ -f "$kv" ] || continue
      ph="$(grep "^phase=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
      if [ "$ph" != "done" ]; then
        active=$((active + 1))
      fi
    done
  fi
  info "pipelines: $active active"

  echo ""
  if [ "$problems" -gt 0 ]; then
    warn "$problems blocking problem(s), $warnings warning(s)."
    return 1
  fi
  if [ "$warnings" -gt 0 ]; then
    info "OK with $warnings warning(s) — pipeline works; some read-only git features degraded."
  else
    info "All checks passed."
  fi
}

cmd_inject() {
  local target_phase="${1:-}"
  local artifact_path="${2:-}"

  if [ -z "$target_phase" ] || [ -z "$artifact_path" ]; then
    die "Usage: pipeline.sh inject <phase> <path>"
  fi

  # Validate target phase
  case "$target_phase" in
    explore|requirements|design|task-plan|implementation|review) ;;
    *) die "Unknown phase: $target_phase. Use: explore, requirements, design, task-plan, implementation, review." ;;
  esac

  resolve_feature || die "No active pipeline. Run 'pipeline.sh init <feature>' first."

  local current_phase
  current_phase="$(read_field phase)"
  [ "$current_phase" = "done" ] && die "Pipeline already complete."

  # Validate current phase <= target phase
  local current_num target_num
  current_num="$(phase_order "$current_phase")"
  target_num="$(phase_order "$target_phase")"
  if [ "$current_num" -gt "$target_num" ]; then
    die "Cannot inject backward: current phase is '$current_phase' ($current_num), target is '$target_phase' ($target_num)."
  fi

  validate_artifact_path "$artifact_path"
  [ -f "$artifact_path" ] || die "Artifact file does not exist: $artifact_path"

  lint_artifact_content "$target_phase" "$artifact_path"

  # Skip intermediate phases (record as injected in history)
  local p="$current_phase"
  local history_count
  history_count="$(read_field history_count)"
  [ -z "$history_count" ] && history_count=0

  while [ "$p" != "$target_phase" ]; do
    write_field "history_${history_count}_phase" "$p"
    write_field "history_${history_count}_artifact" "(injected)"
    write_field "history_${history_count}_approved_at" "$(iso_now)"
    history_count=$((history_count + 1))
    p="$(next_phase "$p")"
  done

  # Set to target phase and register artifact
  write_field phase "$target_phase"
  write_field current_artifact "$artifact_path"
  write_field history_count "$history_count"

  # Save revision snapshot
  local rev_count
  rev_count="$(read_field "revision_count_${target_phase}" || echo "")"
  [ -z "$rev_count" ] && rev_count=0
  rev_count=$((rev_count + 1))
  local rev_name
  rev_name="${target_phase}-rev-${rev_count}-$(iso_now_compact).md"
  cp "$artifact_path" "$REVISIONS_DIR/$rev_name"
  write_field "revision_count_${target_phase}" "$rev_count"

  # Capture review_base_commit if injecting into implementation/review and not already set
  case "$target_phase" in
    implementation|review)
      local rbc
      rbc="$(read_field review_base_commit 2>/dev/null || echo "")"
      if [ -z "$rbc" ]; then
        local head_commit
        head_commit="$(git rev-parse HEAD 2>/dev/null || echo "")"
        if [ -n "$head_commit" ]; then
          write_field review_base_commit "$head_commit"
          info "Captured review_base_commit: $(printf '%.8s' "$head_commit")"
        fi
      fi
      ;;
  esac

  rebuild_json

  local skipped=$((target_num - current_num))
  if [ "$skipped" -gt 0 ]; then
    info "$skipped phase(s) skipped to reach '$target_phase'."
  fi
  info "Artifact injected for phase '$target_phase': $artifact_path"
  info "Ask user to approve, then run 'pipeline.sh approve'"
}

cmd_abandon() {
  local feature="${1:-}"

  # If --feature was specified globally, use it
  if [ -z "$feature" ] && [ -n "$EXPLICIT_FEATURE" ]; then
    feature="$EXPLICIT_FEATURE"
  fi

  # If still empty, try to resolve active feature
  if [ -z "$feature" ]; then
    local rc=0
    feature="$(detect_active_feature)" || rc=$?
    [ "$rc" -eq 2 ] && die "Multiple active pipelines. Use: pipeline.sh abandon <feature-name>"
    [ -z "$feature" ] && die "No active pipeline to abandon."
  fi

  local fdir="$FEATURES_DIR/$feature"
  [ -f "$fdir/pipeline.kv" ] || die "Feature '$feature' not found."

  local phase
  phase="$(grep "^phase=" "$fdir/pipeline.kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
  [ "$phase" = "done" ] && die "Feature '$feature' is already completed."

  set_feature_context "$feature"
  write_field phase "done"
  write_field abandoned_at "$(iso_now)"
  write_field finish_action "abandoned"
  write_field finished_at "$(iso_now)"

  rebuild_json

  info "Feature '$feature' abandoned (was in phase: $phase)."
  info "Artifacts remain in: .spec/features/$feature/"
}

cmd_help() {
  echo "Spec-Driven Dev Pipeline v${VERSION}"
  echo ""
  echo "Usage: sh pipeline.sh [--feature <name>] <command> [args]"
  echo ""
  echo "Global flags:"
  echo "  --feature <name>  Select feature (needed when multiple are active)"
  echo ""
  echo "Commands:"
  echo "  init <feature>    Start a new pipeline (kebab-case name)"
  echo "  status            Show current phase, artifacts, progress"
  echo "  artifact [path]   Register output artifact for current phase"
  echo "  approve           Advance to next phase (needs artifact)"
  echo "  revisions [phase] Show revision history (current phase or specify: explore, all)"
  echo "  history           Show all features and their status"
  echo "  docs-check        Check project documentation status (JSON)"
  echo "  docs-init [--all|--update|<template>...]"
  echo "                    Create standalone docs generation queue"
  echo "                    --all: queue all available templates"
  echo "                    --update: queue only stale templates"
  echo "                    <template>...: queue explicit templates by name"
  echo "  docs-next         Print next pending template (name + path)"
  echo "  docs-done <name>  Mark template as completed in queue"
  echo "  docs-status       Show docs queue progress (JSON)"
  echo "  docs-reset        Clear the docs queue"
  echo "  task-init <T-N>...  Register implementation task set + create tasks/ evidence stubs"
  echo "  task <T-N> [status] Set task status: done (default) | wip | blocked"
  echo "  task-next         Print next actionable task (id + evidence path)"
  echo "  tasks             List tasks with status"
  echo "  task-reset        Clear the task set (keeps evidence files)"
  echo "  config-check      Validate .spec/config.yaml keys and types"
  echo "  doctor            Diagnose environment (tools, templates, .spec, git)"
  echo "  inject <phase> <path>"
  echo "                    Inject pre-written artifact and skip to that phase"
  echo "  abandon [feature] Abandon an active pipeline (marks as done)"
  echo "  version           Show version"
  echo "  help              Show this message"
  echo ""
  echo "Workflow (6 phases):"
  echo "  1. init my-feature"
  echo "  2. (agent reads templates/explore.md, investigates)"
  echo "  3. artifact  ← writes .spec/features/my-feature/explore.md"
  echo "  4. approve   ← user confirms"
  echo "  5. (agent reads templates/requirements.md, generates doc)"
  echo "  6. artifact  ← writes .spec/features/my-feature/requirements.md"
  echo "  7. approve   ← user confirms"
  echo "  8. (agent reads templates/design.md, generates doc)"
  echo "  9. artifact  ← writes .spec/features/my-feature/design.md"
  echo " 10. approve   ← user confirms"
  echo " 11. (agent reads templates/task-plan.md, creates TDD plan)"
  echo " 12. artifact  ← writes .spec/features/my-feature/task-plan.md"
  echo " 13. approve   ← user confirms"
  echo " 14. (agent reads templates/implementation.md, executes TDD plan)"
  echo " 15. artifact  ← writes .spec/features/my-feature/implementation.md"
  echo " 16. approve   ← user confirms"
  echo " 17. (agent reads templates/review.md, reviews code)"
  echo " 18. artifact  ← writes .spec/features/my-feature/review.md"
  echo " 19. approve   ← user confirms → done!"
  echo " 20. docs-check ← update project documentation if needed"
  echo ""
  echo "All artifacts are saved permanently in .spec/features/<feature>/ and tracked by git."
  echo "Tip: use 'revisions' to see previous versions of an artifact within a phase."
}

# --- main ---

# Parse global flags before command dispatch
while [ $# -gt 0 ]; do
  case "$1" in
    --feature)
      [ -n "$2" ] || die "--feature requires a value"
      EXPLICIT_FEATURE="$2"
      shift 2
      ;;
    *) break ;;
  esac
done

case "${1:-help}" in
  init)     shift; cmd_init "$@" ;;
  status)   cmd_status ;;
  artifact) shift; cmd_artifact "$@" ;;
  approve)  cmd_approve ;;
  revisions) shift; cmd_revisions "$@" ;;
  history)  cmd_history ;;
  docs-check) cmd_docs_check ;;
  task)     shift; cmd_task "$@" ;;
  task-init) shift; cmd_task_init "$@" ;;
  task-next) cmd_task_next ;;
  tasks)    cmd_tasks ;;
  task-reset) cmd_task_reset ;;
  config-check) cmd_config_check ;;
  doctor)   cmd_doctor ;;
  inject)   shift; cmd_inject "$@" ;;
  abandon)  shift; cmd_abandon "$@" ;;
  docs-init)   shift; cmd_docs_init "$@" ;;
  docs-next)   cmd_docs_next ;;
  docs-done)   shift; cmd_docs_done "$@" ;;
  docs-status) cmd_docs_status ;;
  docs-reset)  cmd_docs_reset ;;
  version|--version|-v) cmd_version ;;
  help|--help|-h) cmd_help ;;
  *)        die "Unknown command: $1. Run 'pipeline.sh help' for usage." ;;
esac
