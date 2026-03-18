package main

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	weatherv1 "github.com/easyp-tech/protoc-gen-mcp/examples/2-weather-api/proto"
)

type weatherAPI struct{}

func (s *weatherAPI) GetCurrentWeather(ctx context.Context, req *weatherv1.GetCurrentWeatherRequest) (*weatherv1.GetCurrentWeatherResponse, error) {
	var locationName string

	switch loc := req.Location.(type) {
	case *weatherv1.GetCurrentWeatherRequest_City:
		locationName = loc.City
	case *weatherv1.GetCurrentWeatherRequest_Coordinates:
		locationName = fmt.Sprintf("Lat: %f, Lon: %f", loc.Coordinates.Latitude, loc.Coordinates.Longitude)
	default:
		return nil, fmt.Errorf("must provide either city or coordinates")
	}

	condition := "Sunny"
	temp := 22.5
	if len(locationName) > 5 {
		condition = "Cloudy"
		temp = 14.0
	}

	return &weatherv1.GetCurrentWeatherResponse{
		Condition:   condition,
		Temperature: temp,
	}, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "weather-mcp-server",
		Version: "1.0.0",
	}, nil)

	if err := weatherv1.RegisterWeatherAPITools(server, &weatherAPI{}); err != nil {
		log.Fatalf("failed to register tools: %v", err)
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
