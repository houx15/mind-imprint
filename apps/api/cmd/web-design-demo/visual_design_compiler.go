package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// visualPlan is deliberately smaller than VisualDesignSpecV1. The model makes
// design decisions here; compileVisualPlan deterministically supplies the
// repetitive contract fields required by the editable canvas.
type visualPlan struct {
	Title         string                   `json:"title"`
	Brief         visualPlanBrief          `json:"brief"`
	Rationale     string                   `json:"rationale"`
	Theme         visualPlanTheme          `json:"theme"`
	Page          visualPlanPage           `json:"page"`
	ReferenceUses []visualPlanReferenceUse `json:"referenceUses"`
	Assumptions   []string                 `json:"assumptions"`
}

type visualPlanBrief struct {
	Topic       string   `json:"topic"`
	Audience    []string `json:"audience"`
	Goal        string   `json:"goal"`
	Constraints []string `json:"constraints"`
}

type visualPlanTheme struct {
	Background   string    `json:"background"`
	Surfaces     []string  `json:"surfaces"`
	Text         []string  `json:"text"`
	Accents      []string  `json:"accents"`
	HeadingFont  string    `json:"headingFont"`
	BodyFont     string    `json:"bodyFont"`
	BaseFontSize float64   `json:"baseFontSize"`
	SpacingScale []float64 `json:"spacingScale"`
	Radius       float64   `json:"radius"`
}

type visualPlanPage struct {
	Name       string              `json:"name"`
	Width      float64             `json:"width"`
	Height     float64             `json:"height"`
	Background string              `json:"background"`
	Sections   []visualPlanSection `json:"sections"`
	Elements   []visualPlanNode    `json:"elements"`
}

type visualPlanSection struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Name       string           `json:"name"`
	X          float64          `json:"x"`
	Y          float64          `json:"y"`
	Width      float64          `json:"width"`
	Height     float64          `json:"height"`
	Background string           `json:"background"`
	Layout     string           `json:"layout"`
	Columns    int              `json:"columns"`
	Gap        float64          `json:"gap"`
	Padding    float64          `json:"padding"`
	Bounds     []float64        `json:"bounds"`
	Children   []visualPlanNode `json:"children"`
}

type visualPlanNode struct {
	ID                     string    `json:"id"`
	SectionID              string    `json:"sectionId"`
	Type                   string    `json:"type"`
	Name                   string    `json:"name"`
	Text                   string    `json:"text"`
	AssetRef               string    `json:"assetRef"`
	Alt                    string    `json:"alt"`
	X                      float64   `json:"x"`
	Y                      float64   `json:"y"`
	Width                  float64   `json:"width"`
	Height                 float64   `json:"height"`
	Bounds                 []float64 `json:"bounds"`
	Background             string    `json:"background"`
	Color                  string    `json:"color"`
	FontFamily             string    `json:"fontFamily"`
	FontSize               float64   `json:"fontSize"`
	FontWeight             int       `json:"fontWeight"`
	LineHeight             float64   `json:"lineHeight"`
	Radius                 float64   `json:"radius"`
	Action                 string    `json:"action"`
	TargetID               string    `json:"targetId"`
	Href                   string    `json:"href"`
	InteractionDescription string    `json:"interactionDescription"`
}

type visualPlanReferenceUse struct {
	ReferenceID        string   `json:"referenceId"`
	BorrowedPatterns   []string `json:"borrowedPatterns"`
	RejectedPatterns   []string `json:"rejectedPatterns"`
	AffectedSectionIDs []string `json:"affectedSectionIds"`
}

func referenceIDsFromRaw(references []json.RawMessage) []string {
	out := make([]string, 0, len(references))
	seen := make(map[string]bool)
	for _, raw := range references {
		var identity struct {
			ReferenceID string `json:"referenceId"`
		}
		if json.Unmarshal(raw, &identity) != nil {
			continue
		}
		identity.ReferenceID = strings.TrimSpace(identity.ReferenceID)
		if identity.ReferenceID == "" || seen[identity.ReferenceID] {
			continue
		}
		seen[identity.ReferenceID] = true
		out = append(out, identity.ReferenceID)
	}
	return out
}

func parseVisualPlan(text string) (visualPlan, error) {
	value := stripJSONFence(strings.TrimSpace(text))
	var quoted string
	if strings.HasPrefix(value, `"`) && json.Unmarshal([]byte(value), &quoted) == nil {
		value = quoted
	}
	start, end := strings.Index(value, "{"), strings.LastIndex(value, "}")
	if start >= 0 && end > start {
		value = value[start : end+1]
	}
	var plan visualPlan
	if err := json.Unmarshal([]byte(value), &plan); err != nil {
		return visualPlan{}, err
	}
	if err := normalizeFlatVisualPlan(&plan); err != nil {
		return visualPlan{}, err
	}
	return plan, nil
}

func normalizeFlatVisualPlan(plan *visualPlan) error {
	sectionIndex := make(map[string]int, len(plan.Page.Sections))
	for index := range plan.Page.Sections {
		section := &plan.Page.Sections[index]
		if len(section.Bounds) == 4 {
			section.X, section.Y, section.Width, section.Height = section.Bounds[0], section.Bounds[1], section.Bounds[2], section.Bounds[3]
		}
		id := strings.TrimSpace(section.ID)
		if id == "" {
			return fmt.Errorf("page.sections[%d].id is required", index)
		}
		if _, exists := sectionIndex[id]; exists {
			return fmt.Errorf("duplicate section id %q", id)
		}
		sectionIndex[id] = index
	}
	for index := range plan.Page.Elements {
		element := plan.Page.Elements[index]
		if len(element.Bounds) == 4 {
			element.X, element.Y, element.Width, element.Height = element.Bounds[0], element.Bounds[1], element.Bounds[2], element.Bounds[3]
		}
		sectionID := strings.TrimSpace(element.SectionID)
		parentIndex, exists := sectionIndex[sectionID]
		if !exists {
			return fmt.Errorf("page.elements[%d] references unknown sectionId %q", index, sectionID)
		}
		plan.Page.Sections[parentIndex].Children = append(plan.Page.Sections[parentIndex].Children, element)
	}
	return nil
}

func validateVisualPlan(plan visualPlan, referenceIDs []string) error {
	if strings.TrimSpace(plan.Title) == "" || strings.TrimSpace(plan.Brief.Topic) == "" || strings.TrimSpace(plan.Brief.Goal) == "" {
		return errors.New("title, brief.topic and brief.goal are required")
	}
	if len(plan.Page.Sections) < 3 || len(plan.Page.Sections) > 10 {
		return fmt.Errorf("got %d sections, want 3-10", len(plan.Page.Sections))
	}
	nodes, textNodes, interactiveNodes := len(plan.Page.Sections), 0, 0
	for _, section := range plan.Page.Sections {
		nodes += len(section.Children)
		for _, child := range section.Children {
			if strings.TrimSpace(child.Text) != "" {
				textNodes++
			}
			if child.Type == "button" || child.Type == "link" || strings.TrimSpace(child.Action) != "" {
				interactiveNodes++
			}
		}
	}
	if nodes < 12 || nodes > 60 {
		return fmt.Errorf("plan has %d component nodes, want 12-60 according to page complexity", nodes)
	}
	if textNodes < 5 {
		return fmt.Errorf("plan has %d visible text nodes, want at least 5", textNodes)
	}
	if interactiveNodes < 1 {
		return errors.New("plan requires at least one button, link or interaction")
	}
	if len(referenceIDs) > 0 {
		used := make(map[string]bool)
		for _, item := range plan.ReferenceUses {
			if len(item.BorrowedPatterns) > 0 {
				used[item.ReferenceID] = true
			}
		}
		for _, id := range referenceIDs {
			if !used[id] {
				return fmt.Errorf("reference %q has no concrete borrowedPatterns", id)
			}
		}
	}
	return nil
}

var stableIDInvalid = regexp.MustCompile(`[^A-Za-z0-9._:-]+`)

func stableID(prefix, raw string, index int, used map[string]bool) string {
	raw = strings.TrimSpace(raw)
	raw = stableIDInvalid.ReplaceAllString(raw, "-")
	raw = strings.Trim(raw, ".:_-")
	if raw == "" || !isASCIIAlphaNumeric(rune(raw[0])) {
		raw = "item-" + strconv.Itoa(index+1)
	}
	maximumRaw := 160 - len(prefix) - 1
	if len(raw) > maximumRaw {
		raw = strings.Trim(raw[:maximumRaw], ".:_-")
	}
	id := prefix + "." + raw
	base := id
	for suffix := 2; used[id]; suffix++ {
		id = base + "-" + strconv.Itoa(suffix)
	}
	used[id] = true
	return id
}

func isASCIIAlphaNumeric(r rune) bool {
	return r <= unicode.MaxASCII && ((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
}

func finiteOr(value, fallback float64) float64 {
	if value <= 0 || value != value {
		return fallback
	}
	return value
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

var cssColorPattern = regexp.MustCompile(`(?i)^(#[0-9a-f]{3,8}|rgba?\([^;{}]+\)|hsla?\([^;{}]+\)|transparent|currentColor)$`)

func colorOr(value, fallback string) string {
	value = strings.TrimSpace(value)
	if cssColorPattern.MatchString(value) {
		return value
	}
	return fallback
}

func fontOr(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, ";{}") {
		return fallback
	}
	return limitText(value, 160)
}

func compactStrings(values []string, fallback []string, limit int) []string {
	return compactStringsLimited(values, fallback, limit, 500)
}

func compactStringsLimited(values []string, fallback []string, limit, characterLimit int) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		value = limitText(value, characterLimit)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
		if len(out) == limit {
			break
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func compileVisualPlan(plan visualPlan, referenceIDs []string) map[string]any {
	used := make(map[string]bool)
	width := clamp(finiteOr(plan.Page.Width, 1440), 320, 20000)
	height := clamp(finiteOr(plan.Page.Height, 1800), 640, 100000)
	background := colorOr(plan.Page.Background, colorOr(plan.Theme.Background, "#F7F5F2"))
	surfaces := normalizeColors(plan.Theme.Surfaces, []string{"#FFFFFF", "#EEEAE4", "#E4DED5"}, 20)
	texts := normalizeColors(plan.Theme.Text, []string{"#1E2430", "#5D6470"}, 20)
	accents := normalizeColors(plan.Theme.Accents, []string{"#D65A45"}, 20)
	sectionColors := visibleSectionColors(background, surfaces, accents)
	headingFont := fontOr(plan.Theme.HeadingFont, "system-ui")
	bodyFont := fontOr(plan.Theme.BodyFont, "system-ui")
	baseFontSize := clamp(finiteOr(plan.Theme.BaseFontSize, 16), 10, 128)
	radius := clamp(plan.Theme.Radius, 0, 1000)
	if plan.Theme.Radius == 0 {
		radius = 12
	}
	spacing := plan.Theme.SpacingScale
	if len(spacing) == 0 {
		spacing = []float64{8, 16, 24, 32, 48, 64}
	}
	for i := range spacing {
		spacing[i] = clamp(spacing[i], 0, 1000)
	}

	sectionIDs := make(map[string]string)
	nodes := make([]any, 0, 60)
	interactions := make([]any, 0, 12)
	nodeIDs := make(map[string]string)
	previousBottom := float64(0)
	for sectionIndex, section := range plan.Page.Sections {
		sectionID := stableID("node", section.ID, sectionIndex, used)
		sectionIDs[section.ID] = sectionID
		nodeIDs[section.ID] = sectionID
		y := clamp(section.Y, 0, 99999)
		if sectionIndex > 0 && y <= 0 {
			y = previousBottom
		}
		y = clamp(y, 0, 99999)
		x := clamp(section.X, 0, width-1)
		sectionWidth := clamp(finiteOr(section.Width, width-x), 1, width-x)
		sectionHeight := clamp(finiteOr(section.Height, 360), 1, 100000-y)
		if y+sectionHeight > height {
			height = clamp(y+sectionHeight, 640, 100000)
		}
		previousBottom = y + sectionHeight
		sectionType := normalizeSectionType(section.Type, sectionIndex, len(plan.Page.Sections))
		sectionBackground := colorOr(section.Background, sectionColors[sectionIndex%len(sectionColors)])
		layoutMode := section.Layout
		if layoutMode != "flex" && layoutMode != "grid" && layoutMode != "flow" {
			layoutMode = "free"
		}
		layout := map[string]any{"mode": layoutMode}
		if layoutMode == "flex" {
			layout["direction"] = "column"
		}
		if layoutMode == "grid" {
			columns := section.Columns
			if columns < 1 {
				columns = 3
			}
			if columns > 24 {
				columns = 24
			}
			layout["columns"] = columns
		}
		if section.Gap > 0 {
			layout["gap"] = section.Gap
		}
		if section.Padding > 0 {
			layout["padding"] = edgeInsets(section.Padding)
		}
		nodes = append(nodes, map[string]any{
			"id": sectionID, "type": sectionType, "name": limitText(nonEmpty(section.Name, "页面区块 "+strconv.Itoa(sectionIndex+1)), 160),
			"parentId": nil, "order": sectionIndex, "bounds": bounds(x, y, sectionWidth, sectionHeight, sectionIndex),
			"layout": layout, "style": map[string]any{"background": sectionBackground}, "interactionIds": []string{}, "author": "ai",
		})
		for childIndex, child := range section.Children {
			childID := stableID("node", child.ID, len(nodes), used)
			nodeIDs[child.ID] = childID
			childType := normalizeChildType(child.Type, child.Text, child.AssetRef)
			cx := child.X
			cy := child.Y
			// Plans use page-global coordinates. Missing coordinates are laid out
			// in a deterministic grid inside the chosen section.
			if cx == 0 && cy == 0 {
				columns := section.Columns
				if columns < 1 {
					columns = 2
				}
				cellWidth := (sectionWidth - 96) / float64(columns)
				cx = x + 48 + float64(childIndex%columns)*cellWidth
				cy = y + 48 + float64(childIndex/columns)*110
			}
			cx = clamp(cx, x, x+sectionWidth-1)
			cy = clamp(cy, y, y+sectionHeight-1)
			cw := clamp(finiteOr(child.Width, 320), 1, x+sectionWidth-cx)
			ch := clamp(finiteOr(child.Height, defaultNodeHeight(childType)), 1, y+sectionHeight-cy)
			style := map[string]any{}
			if child.Background != "" {
				style["background"] = colorOr(child.Background, surfaces[(sectionIndex+1)%len(surfaces)])
			}
			if child.Color != "" {
				style["color"] = colorOr(child.Color, texts[0])
			}
			if child.Radius > 0 {
				style["borderRadius"] = clamp(child.Radius, 0, 1000)
			} else if childType == "card" || childType == "button" {
				style["borderRadius"] = radius
			}
			if childType == "text" || childType == "button" || childType == "link" || childType == "input" {
				fontSize := clamp(finiteOr(child.FontSize, baseFontSize), 1, 512)
				fontWeight := child.FontWeight
				if fontWeight < 100 || fontWeight > 1000 {
					fontWeight = 400
				}
				lineHeight := clamp(finiteOr(child.LineHeight, 1.5), 0.1, 10)
				style["typography"] = map[string]any{"fontFamily": fontOr(child.FontFamily, chooseFont(fontSize, baseFontSize, headingFont, bodyFont)), "fontSize": fontSize, "fontWeight": fontWeight, "lineHeight": lineHeight}
				if _, ok := style["color"]; !ok {
					style["color"] = texts[0]
				}
			}
			content := map[string]any{}
			if strings.TrimSpace(child.Text) != "" {
				content["text"] = limitText(child.Text, 20000)
			}
			if childType == "image" && strings.TrimSpace(child.AssetRef) != "" {
				content["assetRef"] = limitText(child.AssetRef, 1000)
				content["alt"] = limitText(child.Alt, 500)
			}
			interactionIDs := []string{}
			if childType == "button" || childType == "link" || strings.TrimSpace(child.Action) != "" {
				interactionID := stableID("interaction", child.ID, len(interactions), used)
				action := normalizeAction(child.Action, child.Href, child.TargetID)
				interaction := map[string]any{"id": interactionID, "sourceNodeId": childID, "trigger": "click", "action": action, "description": limitText(nonEmpty(child.InteractionDescription, "执行“"+nonEmpty(child.Text, child.Name)+"”操作"), 500), "confidence": 0.85}
				if (action == "navigate" || action == "open-url") && validHTTPURL(child.Href) && len(child.Href) <= 2048 {
					interaction["href"] = strings.TrimSpace(child.Href)
				}
				interactions = append(interactions, interaction)
				interactionIDs = append(interactionIDs, interactionID)
			}
			node := map[string]any{
				"id": childID, "type": childType, "name": limitText(nonEmpty(child.Name, nonEmpty(child.Text, "组件 "+strconv.Itoa(childIndex+1))), 160),
				"parentId": sectionID, "order": childIndex, "bounds": bounds(cx, cy, cw, ch, childIndex+1),
				"style": style, "interactionIds": interactionIDs, "author": "ai",
			}
			if len(content) > 0 {
				node["content"] = content
			}
			nodes = append(nodes, node)
		}
	}

	// Resolve interaction targets only after every model ID has been mapped.
	interactionIndex := 0
	for _, section := range plan.Page.Sections {
		for _, child := range section.Children {
			if child.Type != "button" && child.Type != "link" && strings.TrimSpace(child.Action) == "" {
				continue
			}
			if interactionIndex >= len(interactions) {
				break
			}
			interaction := interactions[interactionIndex].(map[string]any)
			if target := nodeIDs[child.TargetID]; target != "" && needsTarget(fmt.Sprint(interaction["action"])) {
				interaction["targetNodeId"] = target
			} else if needsTarget(fmt.Sprint(interaction["action"])) {
				interaction["action"] = "custom"
			}
			interactionIndex++
		}
	}

	influences := compileReferenceInfluence(plan.ReferenceUses, referenceIDs, sectionIDs, nodes)
	designID := stableID("design", plan.Title, 0, map[string]bool{})
	pageID := "page.home"
	return map[string]any{
		"schema": "visual-pbl-design-output", "version": "1.0", "designId": designID, "revision": 1, "artifactType": "webpage",
		"title":        limitText(strings.TrimSpace(plan.Title), 300),
		"brief":        map[string]any{"topic": limitText(plan.Brief.Topic, 500), "audience": compactStringsLimited(plan.Brief.Audience, []string{}, 30, 200), "goal": limitText(plan.Brief.Goal, 1000), "constraints": compactStringsLimited(plan.Brief.Constraints, []string{}, 100, 500)},
		"rationale":    limitText(nonEmpty(plan.Rationale, "依据用户要求与参考资料生成可编辑页面结构。"), 5000),
		"theme":        map[string]any{"palette": map[string]any{"background": []string{background}, "surface": surfaces, "text": texts, "accent": accents}, "headingFont": headingFont, "bodyFont": bodyFont, "baseFontSize": baseFontSize, "spacingScale": spacing, "defaultRadius": radius},
		"pages":        []any{map[string]any{"id": pageID, "name": limitText(nonEmpty(plan.Page.Name, "主页"), 160), "viewport": map[string]any{"width": width, "height": height, "device": "desktop"}, "background": background, "nodes": nodes}},
		"interactions": interactions, "referenceInfluence": influences, "assumptions": compactStrings(plan.Assumptions, []string{}, 100),
	}
}

func normalizeColors(values, fallback []string, limit int) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if cssColorPattern.MatchString(strings.TrimSpace(value)) {
			out = append(out, strings.TrimSpace(value))
			if len(out) == limit {
				break
			}
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func visibleSectionColors(background string, surfaces, accents []string) []string {
	values := append([]string{background}, surfaces...)
	values = append(values, accents...)
	values = append(values, "#FFFFFF", "#EEEAE4", "#1E2430")
	out := make([]string, 0, 3)
	seen := make(map[string]bool)
	for _, value := range values {
		key := strings.ToUpper(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
		if len(out) == 3 {
			return out
		}
	}
	return out
}

func edgeInsets(value float64) map[string]any {
	return map[string]any{"top": value, "right": value, "bottom": value, "left": value}
}
func bounds(x, y, width, height float64, z int) map[string]any {
	return map[string]any{"x": x, "y": y, "width": width, "height": height, "zIndex": z}
}
func nonEmpty(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func limitText(value string, maximum int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > maximum {
		runes = runes[:maximum]
	}
	return string(runes)
}

func normalizeSectionType(value string, index, total int) string {
	switch value {
	case "section", "header", "nav", "footer", "group", "grid":
		return value
	}
	if index == 0 {
		return "header"
	}
	if index == total-1 {
		return "footer"
	}
	return "section"
}

func normalizeChildType(value, text, assetRef string) string {
	allowed := map[string]bool{"group": true, "grid": true, "card": true, "text": true, "image": true, "button": true, "link": true, "input": true, "shape": true, "divider": true, "custom": true}
	if allowed[value] {
		return value
	}
	if assetRef != "" {
		return "image"
	}
	if text != "" {
		return "text"
	}
	return "shape"
}

func defaultNodeHeight(nodeType string) float64 {
	if nodeType == "image" || nodeType == "card" || nodeType == "shape" {
		return 180
	}
	return 56
}
func chooseFont(fontSize, base float64, heading, body string) string {
	if fontSize >= base*1.5 {
		return heading
	}
	return body
}

func normalizeAction(action, href, target string) string {
	allowed := map[string]bool{"navigate": true, "open-url": true, "scroll-to": true, "toggle": true, "show": true, "hide": true, "open-modal": true, "submit": true, "custom": true}
	if !allowed[action] {
		if validHTTPURL(href) {
			return "open-url"
		}
		if target != "" {
			return "scroll-to"
		}
		return "custom"
	}
	if (action == "navigate" || action == "open-url") && !validHTTPURL(href) {
		return "custom"
	}
	return action
}

func validHTTPURL(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) > 2048 {
		return false
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func needsTarget(action string) bool {
	return action == "scroll-to" || action == "toggle" || action == "show" || action == "hide" || action == "open-modal"
}

func compileReferenceInfluence(uses []visualPlanReferenceUse, referenceIDs []string, sectionIDs map[string]string, nodes []any) []any {
	byID := make(map[string]visualPlanReferenceUse)
	for _, use := range uses {
		byID[use.ReferenceID] = use
	}
	fallbackIDs := make([]string, 0, 6)
	for _, raw := range nodes {
		node := raw.(map[string]any)
		if node["parentId"] == nil {
			fallbackIDs = append(fallbackIDs, fmt.Sprint(node["id"]))
			if len(fallbackIDs) == 6 {
				break
			}
		}
	}
	out := make([]any, 0, len(referenceIDs))
	for _, referenceID := range referenceIDs {
		use := byID[referenceID]
		affected := make([]string, 0, 6)
		for _, id := range use.AffectedSectionIDs {
			if compiled := sectionIDs[id]; compiled != "" {
				affected = append(affected, compiled)
				if len(affected) == 6 {
					break
				}
			}
		}
		if len(affected) == 0 {
			affected = append(affected, fallbackIDs...)
		}
		out = append(out, map[string]any{"referenceId": referenceID, "borrowedPatterns": compactStrings(use.BorrowedPatterns, []string{"参考资料的视觉层级与布局关系"}, 3), "rejectedPatterns": compactStrings(use.RejectedPatterns, []string{}, 3), "affectedNodeIds": affected})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return fmt.Sprint(out[i].(map[string]any)["referenceId"]) < fmt.Sprint(out[j].(map[string]any)["referenceId"])
	})
	return out
}

