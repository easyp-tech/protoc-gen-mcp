from __future__ import annotations

from pathlib import Path
import sys

_EXAMPLES_ROOT = Path(__file__).resolve().parents[1]
if str(_EXAMPLES_ROOT) not in sys.path:
    sys.path.insert(0, str(_EXAMPLES_ROOT))

import anyio
import mcp.server.lowlevel
import mcp.server.stdio

from proto import weather_mcp


class WeatherAPI:
    def get_current_weather(
        self,
        _ctx: weather_mcp.ToolRequestContext,
        req: weather_mcp.GetCurrentWeatherRequest,
    ) -> weather_mcp.GetCurrentWeatherResponse:
        if req.location is weather_mcp.UNSET:
            raise ValueError("must provide either city or coordinates")
        if isinstance(req.location, weather_mcp.GetCurrentWeatherRequestLocationCityVariant):
            location_name = req.location.city
        elif isinstance(req.location, weather_mcp.GetCurrentWeatherRequestLocationCoordinatesVariant):
            location_name = (
                f"Lat: {req.location.coordinates.latitude:f}, "
                f"Lon: {req.location.coordinates.longitude:f}"
            )
        else:
            raise ValueError("must provide either city or coordinates")

        condition = "Sunny"
        temperature = 22.5
        if len(location_name) > 5:
            condition = "Cloudy"
            temperature = 14.0

        return weather_mcp.GetCurrentWeatherResponse(
            condition=condition,
            temperature=temperature,
        )


def new_server() -> mcp.server.lowlevel.Server:
    server = mcp.server.lowlevel.Server("weather-mcp-server", version="1.0.0")
    weather_mcp.register_weather_api_tools(server, WeatherAPI())
    return server


async def run_stdio_server() -> None:
    server = new_server()
    async with mcp.server.stdio.stdio_server() as (read_stream, write_stream):
        await server.run(
            read_stream,
            write_stream,
            server.create_initialization_options(),
        )


def main() -> None:
    anyio.run(run_stdio_server)


if __name__ == "__main__":
    main()
