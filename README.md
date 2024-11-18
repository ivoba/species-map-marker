# SpeciesMapMarkerCLI - Species Map Marker Generator

## Overview
SpeciesMapMarkerCLI is a command-line tool that generates map markers with species silhouettes from PhyloPic.

## Features
- Fetch species silhouettes from PhyloPic API
- Generate custom map markers with species images
- Normalize species names for accurate searching
- SVG output format for high-quality markers

## Installation
1. Ensure you have Go installed (version 1.21.0 or higher)
2. Clone this repository
3. Run `go mod tidy` to download dependencies

## Usage

### Create a Marker
```bash
go run main.go make-marker "species name"
```

Example:
```bash
go run main.go make-marker "Bufo bufo"
```

This will:
1. Download the species silhouette from PhyloPic
2. Combine it with a map marker template
3. Create new SVG files in the `files` directory:
   - `{species_name}_{uuid}.svg`: Original species silhouette
   - `{species_name}_marker.svg`: Combined map marker

## Building the Application
To build an executable:
```bash
go build -o species-map-marker
```

Then run with:
```bash
./species-map-marker make-marker "Bufo bufo"
```

## Dependencies
- Cobra CLI Framework
- golang.org/x/text for text normalization

## Todo
- gorelease & action