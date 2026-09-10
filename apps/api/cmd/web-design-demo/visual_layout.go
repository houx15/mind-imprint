package main

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
)

// organizeVisualLayout is the deterministic safety pass between an AI plan and
// the canvas. The canvas deliberately uses absolute positions so learners can
// move every component; therefore CSS grid/flex alone cannot prevent overlap.
type layoutBox struct{ x, y, width, height float64 }

func organizeVisualLayout(rawNodes []any, pageWidth float64, pageHeight *float64) {
	nodes := make([]map[string]any, 0, len(rawNodes))
	children := make(map[string][]map[string]any)
	for _, raw := range rawNodes {
		node, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		nodes = append(nodes, node)
		if parentID := stringValue(node["parentId"]); parentID != "" {
			children[parentID] = append(children[parentID], node)
		}
	}
	sections := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		if node["parentId"] == nil && isContainer(stringValue(node["type"])) {
			sections = append(sections, node)
		}
	}
	sortNodesByOrder(sections)
	cursorY := float64(0)
	for _, section := range sections {
		box := readLayoutBox(section)
		box.width = clamp(box.width, 1, pageWidth-box.x)
		box.x = clamp(box.x, 0, pageWidth-1)
		// Sections are page rows. Preserve intentional gaps, but never let two
		// top-level sections occupy the same vertical territory.
		box.y = math.Max(box.y, cursorY)
		box.height = math.Max(box.height, 120)
		sectionID := stringValue(section["id"])
		items := children[sectionID]
		sortNodesByOrder(items)
		padding, gap, mode, columns := sectionLayout(section)
		innerX, innerY := box.x+padding, box.y+padding
		innerWidth := math.Max(1, box.width-padding*2)

		switch mode {
		case "grid":
			layoutGrid(items, innerX, innerY, innerWidth, gap, columns)
		case "flex", "flow":
			layoutFlow(items, innerX, innerY, innerWidth, gap)
		default:
			layoutFree(items, box, padding, gap)
		}

		bottom := box.y + padding
		for _, item := range items {
			itemBox := readLayoutBox(item)
			itemBox.height = math.Max(itemBox.height, requiredTextHeight(item, itemBox.width))
			itemBox.x = clamp(itemBox.x, box.x+padding, box.x+box.width-padding-itemBox.width)
			itemBox.y = math.Max(itemBox.y, box.y+padding)
			writeLayoutBox(item, itemBox)
			bottom = math.Max(bottom, itemBox.y+itemBox.height)
		}
		box.height = math.Max(box.height, bottom-box.y+padding)
		writeLayoutBox(section, box)
		cursorY = box.y + box.height
		if cursorY > *pageHeight {
			*pageHeight = cursorY
		}
	}
}

func isContainer(kind string) bool {
	switch kind {
	case "section", "header", "nav", "footer", "group", "grid":
		return true
	default:
		return false
	}
}

func layoutGrid(items []map[string]any, x, y, width, gap float64, columns int) {
	if columns < 1 {
		columns = 2
	}
	columns = int(clamp(float64(columns), 1, 6))
	cellWidth := math.Max(48, (width-gap*float64(columns-1))/float64(columns))
	rowY, rowHeight := y, float64(0)
	for index, item := range items {
		if index > 0 && index%columns == 0 {
			rowY += rowHeight + gap
			rowHeight = 0
		}
		box := readLayoutBox(item)
		box.width = cellWidth
		box.height = math.Max(box.height, requiredTextHeight(item, cellWidth))
		box.x = x + float64(index%columns)*(cellWidth+gap)
		box.y = rowY
		writeLayoutBox(item, box)
		rowHeight = math.Max(rowHeight, box.height)
	}
}

func layoutFlow(items []map[string]any, x, y, width, gap float64) {
	cursor := y
	for _, item := range items {
		box := readLayoutBox(item)
		box.width = math.Min(math.Max(48, box.width), width)
		box.height = math.Max(box.height, requiredTextHeight(item, box.width))
		box.x, box.y = x, cursor
		writeLayoutBox(item, box)
		cursor += box.height + gap
	}
}

func layoutFree(items []map[string]any, section layoutBox, padding, gap float64) {
	placed := make([]layoutBox, 0, len(items))
	for _, item := range items {
		box := readLayoutBox(item)
		// Plans are documented as page-global, but references and model outputs
		// often use local section coordinates. Values above the section are safely
		// interpreted as local instead of being stacked at the section origin.
		if box.y < section.y {
			box.y += section.y
		}
		if box.x < section.x {
			box.x += section.x
		}
		box.width = math.Min(math.Max(36, box.width), math.Max(36, section.width-padding*2))
		box.height = math.Max(box.height, requiredTextHeight(item, box.width))
		box.x = clamp(box.x, section.x+padding, section.x+section.width-padding-box.width)
		box.y = math.Max(box.y, section.y+padding)
		for guard := 0; guard < 120; guard++ {
			hit := false
			for _, previous := range placed {
				if boxesOverlap(box, previous, gap) {
					box.y = previous.y + previous.height + gap
					hit = true
				}
			}
			if !hit {
				break
			}
		}
		writeLayoutBox(item, box)
		placed = append(placed, box)
	}
}

func boxesOverlap(a, b layoutBox, gap float64) bool {
	return a.x < b.x+b.width+gap && a.x+a.width+gap > b.x && a.y < b.y+b.height+gap && a.y+a.height+gap > b.y
}

func sectionLayout(node map[string]any) (padding, gap float64, mode string, columns int) {
	padding, gap, mode, columns = 48, 24, "free", 2
	layout, _ := node["layout"].(map[string]any)
	if layout == nil {
		return
	}
	if value := numberValue(layout["gap"]); value > 0 {
		gap = clamp(value, 4, 120)
	}
	if value := numberValue(layout["columns"]); value > 0 {
		columns = int(value)
	}
	if value := stringValue(layout["mode"]); value == "grid" || value == "flex" || value == "flow" {
		mode = value
	}
	if inset, ok := layout["padding"].(map[string]any); ok {
		if value := numberValue(inset["left"]); value > 0 {
			padding = clamp(value, 0, 240)
		}
	}
	return
}

func requiredTextHeight(node map[string]any, width float64) float64 {
	content, _ := node["content"].(map[string]any)
	text := strings.TrimSpace(stringValue(content["text"]))
	if text == "" || width <= 0 {
		return 0
	}
	fontSize, lineHeight := 16.0, 1.5
	if style, ok := node["style"].(map[string]any); ok {
		if typography, ok := style["typography"].(map[string]any); ok {
			if value := numberValue(typography["fontSize"]); value > 0 {
				fontSize = value
			}
			if value := numberValue(typography["lineHeight"]); value > 0 {
				lineHeight = value
			}
		}
	}
	perLine := math.Max(4, math.Floor((width-20)/(fontSize*0.72)))
	lines := float64(0)
	for _, line := range strings.Split(text, "\n") {
		lines += math.Max(1, math.Ceil(float64(len([]rune(line))/int(perLine))))
	}
	return math.Ceil(lines*fontSize*lineHeight + 20)
}

func readLayoutBox(node map[string]any) layoutBox {
	bounds, _ := node["bounds"].(map[string]any)
	return layoutBox{x: numberValue(bounds["x"]), y: numberValue(bounds["y"]), width: math.Max(1, numberValue(bounds["width"])), height: math.Max(1, numberValue(bounds["height"]))}
}

func writeLayoutBox(node map[string]any, box layoutBox) {
	bounds, _ := node["bounds"].(map[string]any)
	if bounds == nil {
		bounds = map[string]any{}
		node["bounds"] = bounds
	}
	bounds["x"], bounds["y"], bounds["width"], bounds["height"] = box.x, box.y, box.width, box.height
}

func sortNodesByOrder(nodes []map[string]any) {
	sort.SliceStable(nodes, func(i, j int) bool { return numberValue(nodes[i]["order"]) < numberValue(nodes[j]["order"]) })
}

func numberValue(value any) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case float32:
		return float64(number)
	case int:
		return float64(number)
	case int64:
		return float64(number)
	case json.Number:
		parsed, _ := number.Float64()
		return parsed
	default:
		return 0
	}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}
