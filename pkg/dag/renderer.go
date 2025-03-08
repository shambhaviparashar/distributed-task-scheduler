package dag

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// RenderFormat defines the output format for graph rendering
type RenderFormat string

const (
	// FormatDOT generates a DOT format output (for Graphviz)
	FormatDOT RenderFormat = "dot"
	// FormatJSON generates a JSON format for web visualization
	FormatJSON RenderFormat = "json"
	// FormatMermaid generates a Mermaid diagram
	FormatMermaid RenderFormat = "mermaid"
)

// RenderOptions contains configuration options for graph rendering
type RenderOptions struct {
	Format          RenderFormat     // Output format
	Title           string           // Graph title
	IncludeMetadata bool             // Whether to include metadata in the output
	NodeStyles      map[string]Style // Styles for nodes based on status
	EdgeStyles      map[string]Style // Styles for edges based on type
	HighlightNodes  []string         // Node IDs to highlight
	Layout          string           // Layout algorithm (for DOT format)
}

// Style represents visual styling attributes
type Style struct {
	Color       string
	Shape       string
	Style       string
	Label       string
	FontColor   string
	ArrowHead   string
	ArrowTail   string
	StrokeWidth string
}

// DefaultRenderOptions returns the default rendering options
func DefaultRenderOptions() RenderOptions {
	return RenderOptions{
		Format: FormatDOT,
		Title:  "Task Dependency Graph",
		NodeStyles: map[string]Style{
			"pending": {
				Color: "gray",
				Shape: "box",
				Style: "filled",
			},
			"running": {
				Color: "blue",
				Shape: "box",
				Style: "filled",
			},
			"completed": {
				Color: "green",
				Shape: "box",
				Style: "filled",
			},
			"failed": {
				Color: "red",
				Shape: "box",
				Style: "filled",
			},
			"cancelled": {
				Color: "orange",
				Shape: "box",
				Style: "filled",
			},
			"default": {
				Color: "black",
				Shape: "box",
				Style: "solid",
			},
		},
		EdgeStyles: map[string]Style{
			"dependency": {
				Color:     "black",
				Style:     "solid",
				ArrowHead: "normal",
			},
			"conditional": {
				Color:     "blue",
				Style:     "dashed",
				ArrowHead: "diamond",
			},
			"default": {
				Color:     "gray",
				Style:     "solid",
				ArrowHead: "normal",
			},
		},
		Layout: "dot",
	}
}

// Render renders the graph to the specified writer in the desired format
func (g *Graph) Render(w io.Writer, opts RenderOptions) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	switch opts.Format {
	case FormatDOT:
		return g.renderDOT(w, opts)
	case FormatJSON:
		return g.renderJSON(w, opts)
	case FormatMermaid:
		return g.renderMermaid(w, opts)
	default:
		return fmt.Errorf("unsupported render format: %s", opts.Format)
	}
}

// renderDOT renders the graph in DOT format for Graphviz
func (g *Graph) renderDOT(w io.Writer, opts RenderOptions) error {
	var buf bytes.Buffer

	// Write the header
	buf.WriteString(fmt.Sprintf("digraph \"%s\" {\n", opts.Title))
	buf.WriteString(fmt.Sprintf("  layout=%s;\n", opts.Layout))
	buf.WriteString("  node [fontname=\"Arial\"];\n")
	buf.WriteString("  edge [fontname=\"Arial\"];\n\n")

	// Write the nodes
	for id, node := range g.Nodes {
		style := opts.NodeStyles["default"]
		if s, ok := opts.NodeStyles[node.Status]; ok {
			style = s
		}

		// Handle highlighted nodes
		isHighlighted := false
		for _, highlightID := range opts.HighlightNodes {
			if id == highlightID {
				isHighlighted = true
				break
			}
		}

		if isHighlighted {
			style.StrokeWidth = "2"
			style.Style = "bold,filled"
		}

		// Prepare the label
		label := node.Name
		if opts.IncludeMetadata && len(node.Metadata) > 0 {
			label += "\\n"
			for k, v := range node.Metadata {
				label += fmt.Sprintf("%s: %v\\n", k, v)
			}
		}

		buf.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\", shape=%s, style=\"%s\", fillcolor=\"%s\"];\n",
			id, label, style.Shape, style.Style, style.Color))
	}

	buf.WriteString("\n")

	// Write the edges
	for from, edges := range g.Edges {
		for _, edge := range edges {
			style := opts.EdgeStyles["default"]
			if s, ok := opts.EdgeStyles[edge.Type]; ok {
				style = s
			}

			label := ""
			if opts.IncludeMetadata && len(edge.Metadata) > 0 {
				labels := make([]string, 0, len(edge.Metadata))
				for k, v := range edge.Metadata {
					labels = append(labels, fmt.Sprintf("%s: %v", k, v))
				}
				label = strings.Join(labels, "\\n")
			}

			edgeAttrs := fmt.Sprintf("color=\"%s\", style=\"%s\", arrowhead=\"%s\"", 
				style.Color, style.Style, style.ArrowHead)
			
			if label != "" {
				edgeAttrs += fmt.Sprintf(", label=\"%s\"", label)
			}

			buf.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [%s];\n", from, edge.To, edgeAttrs))
		}
	}

	buf.WriteString("}\n")

	_, err := w.Write(buf.Bytes())
	return err
}

// renderMermaid renders the graph in Mermaid.js format
func (g *Graph) renderMermaid(w io.Writer, opts RenderOptions) error {
	var buf bytes.Buffer

	// Write the header
	buf.WriteString("graph TD;\n")
	
	// Add title if provided
	if opts.Title != "" {
		buf.WriteString(fmt.Sprintf("  %% %s\n", opts.Title))
	}

	// Process nodes
	for id, node := range g.Nodes {
		label := node.Name
		if opts.IncludeMetadata && len(node.Metadata) > 0 {
			label += "<br>"
			for k, v := range node.Metadata {
				label += fmt.Sprintf("%s: %v<br>", k, v)
			}
		}

		// Style class based on status
		styleClass := "default"
		if node.Status != "" {
			styleClass = node.Status
		}

		// Write the node
		buf.WriteString(fmt.Sprintf("  %s[\"%s\"]:::%s;\n", id, label, styleClass))
	}

	// Process edges
	for from, edges := range g.Edges {
		for _, edge := range edges {
			edgeType := "-->"
			if edge.Type == "conditional" {
				edgeType = "-.->"; // Dashed edge for conditional dependencies
			}

			label := ""
			if opts.IncludeMetadata && len(edge.Metadata) > 0 {
				labels := make([]string, 0, len(edge.Metadata))
				for k, v := range edge.Metadata {
					labels = append(labels, fmt.Sprintf("%s: %v", k, v))
				}
				label = strings.Join(labels, "<br>")
				buf.WriteString(fmt.Sprintf("  %s %s|%s| %s;\n", from, edgeType, label, edge.To))
			} else {
				buf.WriteString(fmt.Sprintf("  %s %s %s;\n", from, edgeType, edge.To))
			}
		}
	}

	// Add style definitions
	buf.WriteString("\n  %% Style definitions\n")
	
	// Node style classes
	for status, style := range opts.NodeStyles {
		color := style.Color
		if color == "" {
			color = "white"
		}
		
		buf.WriteString(fmt.Sprintf("  classDef %s fill:%s,stroke:%s,color:black;\n", 
			status, color, style.Color))
	}

	_, err := w.Write(buf.Bytes())
	return err
}

// renderJSON renders the graph in JSON format for web-based visualization
func (g *Graph) renderJSON(w io.Writer, opts RenderOptions) error {
	var buf bytes.Buffer

	// Start the JSON object
	buf.WriteString("{\n")
	buf.WriteString(fmt.Sprintf("  \"title\": \"%s\",\n", opts.Title))
	buf.WriteString("  \"nodes\": [\n")

	// Write nodes
	nodeCount := len(g.Nodes)
	i := 0
	for id, node := range g.Nodes {
		i++
		
		// Define the node style
		style := opts.NodeStyles["default"]
		if s, ok := opts.NodeStyles[node.Status]; ok {
			style = s
		}

		// Check if the node is highlighted
		isHighlighted := false
		for _, highlightID := range opts.HighlightNodes {
			if id == highlightID {
				isHighlighted = true
				break
			}
		}

		// Write the node object
		buf.WriteString("    {\n")
		buf.WriteString(fmt.Sprintf("      \"id\": \"%s\",\n", id))
		buf.WriteString(fmt.Sprintf("      \"name\": \"%s\",\n", node.Name))
		buf.WriteString(fmt.Sprintf("      \"status\": \"%s\",\n", node.Status))
		
		// Include metadata if specified
		if opts.IncludeMetadata && len(node.Metadata) > 0 {
			buf.WriteString("      \"metadata\": {\n")
			metaCount := len(node.Metadata)
			j := 0
			for k, v := range node.Metadata {
				j++
				buf.WriteString(fmt.Sprintf("        \"%s\": \"%v\"%s\n", 
					k, v, if j < metaCount { "," } else { "" }))
			}
			buf.WriteString("      },\n")
		}

		// Add styling information
		buf.WriteString("      \"style\": {\n")
		buf.WriteString(fmt.Sprintf("        \"color\": \"%s\",\n", style.Color))
		buf.WriteString(fmt.Sprintf("        \"shape\": \"%s\",\n", style.Shape))
		buf.WriteString(fmt.Sprintf("        \"style\": \"%s\",\n", style.Style))
		buf.WriteString(fmt.Sprintf("        \"highlighted\": %t\n", isHighlighted))
		buf.WriteString("      }\n")
		
		// End the node object
		if i < nodeCount {
			buf.WriteString("    },\n")
		} else {
			buf.WriteString("    }\n")
		}
	}

	buf.WriteString("  ],\n")
	buf.WriteString("  \"edges\": [\n")

	// Write edges
	edgeCount := 0
	for _, edges := range g.Edges {
		edgeCount += len(edges)
	}

	currentEdge := 0
	for from, edges := range g.Edges {
		for _, edge := range edges {
			currentEdge++
			
			// Define the edge style
			style := opts.EdgeStyles["default"]
			if s, ok := opts.EdgeStyles[edge.Type]; ok {
				style = s
			}

			// Write the edge object
			buf.WriteString("    {\n")
			buf.WriteString(fmt.Sprintf("      \"from\": \"%s\",\n", from))
			buf.WriteString(fmt.Sprintf("      \"to\": \"%s\",\n", edge.To))
			buf.WriteString(fmt.Sprintf("      \"type\": \"%s\",\n", edge.Type))
			
			// Include metadata if specified
			if opts.IncludeMetadata && len(edge.Metadata) > 0 {
				buf.WriteString("      \"metadata\": {\n")
				metaCount := len(edge.Metadata)
				j := 0
				for k, v := range edge.Metadata {
					j++
					buf.WriteString(fmt.Sprintf("        \"%s\": \"%v\"%s\n", 
						k, v, if j < metaCount { "," } else { "" }))
				}
				buf.WriteString("      },\n")
			}

			// Add styling information
			buf.WriteString("      \"style\": {\n")
			buf.WriteString(fmt.Sprintf("        \"color\": \"%s\",\n", style.Color))
			buf.WriteString(fmt.Sprintf("        \"style\": \"%s\",\n", style.Style))
			buf.WriteString(fmt.Sprintf("        \"arrowHead\": \"%s\"\n", style.ArrowHead))
			buf.WriteString("      }\n")
			
			// End the edge object
			if currentEdge < edgeCount {
				buf.WriteString("    },\n")
			} else {
				buf.WriteString("    }\n")
			}
		}
	}

	buf.WriteString("  ]\n")
	buf.WriteString("}\n")

	_, err := w.Write(buf.Bytes())
	return err
}

// RenderToString renders the graph to a string in the desired format
func (g *Graph) RenderToString(opts RenderOptions) (string, error) {
	var buf bytes.Buffer
	err := g.Render(&buf, opts)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
} 