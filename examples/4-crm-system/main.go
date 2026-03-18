package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	crmv1 "github.com/easyp-tech/protoc-gen-mcp/examples/4-crm-system/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type crmAPI struct {
	mu    sync.RWMutex
	users map[string]*crmv1.User
}

func newCRMAPI() *crmAPI {
	return &crmAPI{
		users: map[string]*crmv1.User{
			"usr_1": {
				Id:           "usr_1",
				Name:         "Alice Smith",
				Tags:         []string{"premium", "enterprise"},
				RegisteredAt: timestamppb.New(time.Now().Add(-24 * time.Hour)),
			},
			"usr_2": {
				Id:           "usr_2",
				Name:         "Bob Jones",
				Tags:         []string{"basic"},
				RegisteredAt: timestamppb.New(time.Now().Add(-2 * time.Hour)),
			},
		},
	}
}

func (s *crmAPI) ListUsers(ctx context.Context, req *crmv1.ListUsersRequest) (*crmv1.ListUsersResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*crmv1.User
	for _, u := range s.users {
		match := true
		for _, requiredTag := range req.RequiredTags {
			hasTag := false
			for _, tag := range u.Tags {
				if tag == requiredTag {
					hasTag = true
					break
				}
			}
			if !hasTag {
				match = false
				break
			}
		}
		if match {
			result = append(result, u)
		}
	}

	limit := int(req.GetLimit())
	if limit == 0 {
		limit = 10
	}
	if len(result) > limit {
		result = result[:limit]
	}

	return &crmv1.ListUsersResponse{Users: result}, nil
}

func (s *crmAPI) UpdateUser(ctx context.Context, req *crmv1.UpdateUserRequest) (*crmv1.UpdateUserResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userToUpdate := req.GetUser()
	if userToUpdate == nil || userToUpdate.Id == "" {
		return nil, fmt.Errorf("user and user.id must be provided")
	}

	existingUser, ok := s.users[userToUpdate.Id]
	if !ok {
		return nil, fmt.Errorf("user %q not found", userToUpdate.Id)
	}

	if len(req.GetUpdateMask().GetPaths()) == 0 {
		existingUser.Name = userToUpdate.Name
		existingUser.Tags = userToUpdate.Tags
	} else {
		for _, path := range req.UpdateMask.Paths {
			switch path {
			case "name":
				existingUser.Name = userToUpdate.Name
			case "tags":
				existingUser.Tags = userToUpdate.Tags
			}
		}
	}

	s.users[existingUser.Id] = existingUser
	return &crmv1.UpdateUserResponse{User: existingUser}, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "crm-mcp-server",
		Version: "1.0.0",
	}, nil)

	if err := crmv1.RegisterUsersAPITools(server, newCRMAPI()); err != nil {
		log.Fatalf("failed to register tools: %v", err)
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
