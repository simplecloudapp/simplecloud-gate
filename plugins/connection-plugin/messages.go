package connection

import (
	"strings"

	"go.minekube.com/common/minecraft/color"
	"go.minekube.com/common/minecraft/component"
)

type styleFrame struct {
	tag   string
	style component.Style
}

func messageComponent(message string, variables map[string]string) component.Component {
	expanded := expandVariables(message, variables)
	root := &component.Text{}
	style := component.Style{Color: color.White}
	stack := make([]styleFrame, 0)

	appendText := func(content string) {
		if content == "" {
			return
		}
		root.Extra = append(root.Extra, &component.Text{Content: content, S: style})
	}

	for len(expanded) > 0 {
		open := strings.IndexByte(expanded, '<')
		if open < 0 {
			appendText(expanded)
			break
		}
		appendText(expanded[:open])
		closeOffset := strings.IndexByte(expanded[open:], '>')
		if closeOffset < 0 {
			appendText(expanded[open:])
			break
		}
		closeIndex := open + closeOffset
		rawTag := expanded[open+1 : closeIndex]
		expanded = expanded[closeIndex+1:]
		tag := strings.ToLower(strings.TrimSpace(rawTag))

		if tag == "br" || tag == "newline" {
			appendText("\n")
			continue
		}
		if tag == "reset" {
			style = component.Style{Color: color.White}
			stack = stack[:0]
			continue
		}
		if strings.HasPrefix(tag, "/") {
			closing := strings.TrimPrefix(tag, "/")
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].tag == closing {
					style = stack[i].style
					stack = stack[:i]
					break
				}
			}
			continue
		}

		previous := style
		switch {
		case strings.HasPrefix(tag, "color:"):
			parsed, ok := parseColor(strings.TrimPrefix(tag, "color:"))
			if !ok {
				appendText("<" + rawTag + ">")
				continue
			}
			stack = append(stack, styleFrame{tag: "color", style: previous})
			style.Color = parsed
		case strings.HasPrefix(tag, "#"):
			parsed, ok := parseColor(tag)
			if !ok {
				appendText("<" + rawTag + ">")
				continue
			}
			stack = append(stack, styleFrame{tag: tag, style: previous})
			style.Color = parsed
		case tag == "bold" || tag == "b":
			stack = append(stack, styleFrame{tag: tag, style: previous})
			style.Bold = component.True
		case tag == "italic" || tag == "i":
			stack = append(stack, styleFrame{tag: tag, style: previous})
			style.Italic = component.True
		case tag == "underlined" || tag == "u":
			stack = append(stack, styleFrame{tag: tag, style: previous})
			style.Underlined = component.True
		case tag == "strikethrough" || tag == "st":
			stack = append(stack, styleFrame{tag: tag, style: previous})
			style.Strikethrough = component.True
		case tag == "obfuscated" || tag == "obf":
			stack = append(stack, styleFrame{tag: tag, style: previous})
			style.Obfuscated = component.True
		default:
			if parsed, ok := parseColor(tag); ok {
				stack = append(stack, styleFrame{tag: tag, style: previous})
				style.Color = parsed
			} else {
				appendText("<" + rawTag + ">")
			}
		}
	}
	return root
}

func expandVariables(message string, variables map[string]string) string {
	for pass := 0; pass < 10; pass++ {
		previous := message
		for name, value := range variables {
			message = strings.ReplaceAll(message, "<"+name+">", value)
		}
		if message == previous {
			break
		}
	}
	return message
}

func parseColor(name string) (color.Color, bool) {
	if strings.HasPrefix(name, "#") {
		parsed, err := color.Hex(name)
		return parsed, err == nil
	}
	parsed, ok := color.Names[strings.ToLower(name)]
	return parsed, ok
}
