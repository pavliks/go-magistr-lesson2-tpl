package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: ./yamlvalid <path>")
		os.Exit(1)
	}

	filePath := os.Args[1]

	base := filepath.Base(filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read file content: %v\n", err)
		os.Exit(1)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		fmt.Fprintf(os.Stderr, "cannot unmarshal file content: %v\n", err)
		os.Exit(1)
	}

	if len(root.Content) == 0 {
		return
	}

	doc := root.Content[0]
	errorsFound := 0

	regexSnake := regexp.MustCompile(`^[a-z0-9]+(?:_[a-z0-9]+)*$`)
	regexMemory := regexp.MustCompile(`^[0-9]+(?:Ki|Mi|Gi)$`)
	regexImage := regexp.MustCompile(`^registry\.bigbrother\.io/[A-Za-z0-9._/-]+:[A-Za-z0-9._-]+$`)

	var nodeAPIVersion, nodeKind, nodeMetadata, nodeSpec *yaml.Node

	for i := 0; i+1 < len(doc.Content); i += 2 {
		keyNode := doc.Content[i]
		valueNode := doc.Content[i+1]
		switch keyNode.Value {
		case "apiVersion":
			nodeAPIVersion = valueNode
		case "kind":
			nodeKind = valueNode
		case "metadata":
			nodeMetadata = valueNode
		case "spec":
			nodeSpec = valueNode
		}
	}

	if nodeAPIVersion == nil {
		fmt.Printf("%s: apiVersion is required\n", base)
		errorsFound++
	}
	if nodeKind == nil {
		fmt.Printf("%s: kind is required\n", base)
		errorsFound++
	}
	if nodeMetadata == nil {
		fmt.Printf("%s: metadata is required\n", base)
		errorsFound++
	}
	if nodeSpec == nil {
		fmt.Printf("%s: spec is required\n", base)
		errorsFound++
	}

	var metaName *yaml.Node
	if nodeMetadata != nil && nodeMetadata.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(nodeMetadata.Content); i += 2 {
			key := nodeMetadata.Content[i]
			value := nodeMetadata.Content[i+1]
			if key.Value == "name" {
				metaName = value
			}
		}
	}

	if metaName == nil || strings.TrimSpace(metaName.Value) == "" {
		fmt.Printf("%s: name is required\n", base)
		errorsFound++
	}

	var specOS *yaml.Node
	var specContainers *yaml.Node
	if nodeSpec != nil && nodeSpec.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(nodeSpec.Content); i += 2 {
			key := nodeSpec.Content[i]
			value := nodeSpec.Content[i+1]
			switch key.Value {
			case "os":
				specOS = value
			case "containers":
				specContainers = value
			}
		}
	}

	if specOS != nil {
		if specOS.Kind == yaml.ScalarNode {
			value := strings.TrimSpace(specOS.Value)
			if value != "linux" && value != "windows" {
				fmt.Printf("%s:%d os has unsupported value '%s'\n", base, specOS.Line, specOS.Value)
				errorsFound++
			}
		} else if specOS.Kind == yaml.MappingNode {
			var osName *yaml.Node
			for i := 0; i+1 < len(specOS.Content); i += 2 {
				key := specOS.Content[i]
				value := specOS.Content[i+1]
				if key.Value == "name" {
					osName = value
				}
			}
			if osName == nil || strings.TrimSpace(osName.Value) == "" {
				fmt.Printf("%s: name is required\n", base)
				errorsFound++
			} else {
				if osName.Value != "linux" && osName.Value != "windows" {
					fmt.Printf("%s:%d os has unsupported value '%s'\n", base, osName.Line, osName.Value)
					errorsFound++
				}
			}
		}
	}

	if specContainers == nil {
		fmt.Printf("%s: containers is required\n", base)
		errorsFound++
	} else if specContainers.Kind != yaml.SequenceNode || len(specContainers.Content) == 0 {
		fmt.Printf("%s: containers is required\n", base)
		errorsFound++
	} else {
		for _, containerNode := range specContainers.Content {
			if containerNode.Kind != yaml.MappingNode {
				continue
			}
			var containerName, containerImage, containerResources *yaml.Node
			var containerPorts *yaml.Node
			var containerReadiness, containerLiveness *yaml.Node

			for i := 0; i+1 < len(containerNode.Content); i += 2 {
				key := containerNode.Content[i]
				value := containerNode.Content[i+1]
				switch key.Value {
				case "name":
					containerName = value
				case "image":
					containerImage = value
				case "ports":
					containerPorts = value
				case "readinessProbe":
					containerReadiness = value
				case "livenessProbe":
					containerLiveness = value
				case "resources":
					containerResources = value
				}
			}

			if containerName == nil || strings.TrimSpace(containerName.Value) == "" {
				fmt.Printf("%s: name is required\n", base)
				errorsFound++
			} else {
				if !regexSnake.MatchString(containerName.Value) {
					fmt.Printf("%s:%d name has invalid format '%s'\n", base, containerName.Line, containerName.Value)
					errorsFound++
				}
			}

			if containerImage == nil || strings.TrimSpace(containerImage.Value) == "" {
				fmt.Printf("%s: image is required\n", base)
				errorsFound++
			} else if !regexImage.MatchString(containerImage.Value) {
				fmt.Printf("%s:%d image has invalid format '%s'\n", base, containerImage.Line, containerImage.Value)
				errorsFound++
			}

			if containerPorts != nil && containerPorts.Kind == yaml.SequenceNode {
				for _, port := range containerPorts.Content {
					if port.Kind != yaml.MappingNode {
						continue
					}
					var portNode, protocolNode *yaml.Node
					for i := 0; i+1 < len(port.Content); i += 2 {
						key := port.Content[i]
						value := port.Content[i+1]
						switch key.Value {
						case "containerPort":
							portNode = value
						case "protocol":
							protocolNode = value
						}
					}
					if portNode != nil {
						if portNode.Tag == "!!str" {
							fmt.Printf("%s:%d containerPort must be int\n", base, portNode.Line)
							errorsFound++
						} else {
							portValue, _ := strconv.Atoi(strings.TrimSpace(portNode.Value))
							if portValue <= 0 || portValue > 65535 {
								fmt.Printf("%s:%d containerPort value out of range\n", base, portNode.Line)
								errorsFound++
							}
						}
					} else {
						fmt.Printf("%s: containerPort is required\n", base)
						errorsFound++
					}
					if protocolNode != nil {
						proto := strings.TrimSpace(protocolNode.Value)
						if proto != "" && proto != "TCP" && proto != "UDP" {
							fmt.Printf("%s:%d protocol has unsupported value '%s'\n", base, protocolNode.Line, protocolNode.Value)
							errorsFound++
						}
					}
				}
			}

			checkProbe := func(probe *yaml.Node, field string) {
				if probe == nil || probe.Kind != yaml.MappingNode {
					return
				}
				var httpGet *yaml.Node
				for i := 0; i+1 < len(probe.Content); i += 2 {
					key := probe.Content[i]
					value := probe.Content[i+1]
					if key.Value == "httpGet" {
						httpGet = value
					}
				}
				if httpGet == nil {
					fmt.Printf("%s: httpGet is required\n", base)
					errorsFound++
					return
				}
				if httpGet.Kind == yaml.MappingNode {
					var pathNode, portNode *yaml.Node
					for i := 0; i+1 < len(httpGet.Content); i += 2 {
						key := httpGet.Content[i]
						value := httpGet.Content[i+1]
						if key.Value == "path" {
							pathNode = value
						}
						if key.Value == "port" {
							portNode = value
						}
					}
					if pathNode == nil || !strings.HasPrefix(strings.TrimSpace(pathNode.Value), "/") {
						if pathNode == nil {
							fmt.Printf("%s: path is required\n", base)
						} else {
							fmt.Printf("%s:%d path has invalid format '%s'\n", base, pathNode.Line, pathNode.Value)
						}
						errorsFound++
					}
					if portNode == nil {
						fmt.Printf("%s: port is required\n", base)
						errorsFound++
					} else {
						if portNode.Tag == "!!str" {
							fmt.Printf("%s:%d port must be int\n", base, portNode.Line)
							errorsFound++
						} else {
							portValue, _ := strconv.Atoi(strings.TrimSpace(portNode.Value))
							if portValue <= 0 || portValue > 65535 {
								fmt.Printf("%s:%d port value out of range\n", base, portNode.Line)
								errorsFound++
							}
						}
					}
				}
			}

			checkProbe(containerReadiness, "readinessProbe")
			checkProbe(containerLiveness, "livenessProbe")

			if containerResources == nil {
				fmt.Printf("%s: resources is required\n", base)
				errorsFound++
			} else {
				if containerResources.Kind == yaml.MappingNode {
					for i := 0; i+1 < len(containerResources.Content); i += 2 {

						value := containerResources.Content[i+1]
						if value.Kind != yaml.MappingNode {
							continue
						}
						var cpuNode, memoryNode *yaml.Node
						for j := 0; j+1 < len(value.Content); j += 2 {
							key2 := value.Content[j]
							value2 := value.Content[j+1]
							if key2.Value == "cpu" {
								cpuNode = value2
							}
							if key2.Value == "memory" {
								memoryNode = value2
							}
						}
						if cpuNode != nil {
							if cpuNode.Tag == "!!str" {
								fmt.Printf("%s:%d cpu must be int\n", base, cpuNode.Line)
								errorsFound++
							}
						}
						if memoryNode != nil {
							if memoryNode.Tag != "!!str" || !regexMemory.MatchString(strings.TrimSpace(memoryNode.Value)) {
								fmt.Printf("%s:%d memory has invalid format '%s'\n", base, memoryNode.Line, memoryNode.Value)
								errorsFound++
							}
						}
					}
				}
			}
		}
	}

	if nodeAPIVersion != nil && strings.TrimSpace(nodeAPIVersion.Value) != "v1" {
		fmt.Printf("%s:%d apiVersion has unsupported value '%s'\n", base, nodeAPIVersion.Line, nodeAPIVersion.Value)
		errorsFound++
	}
	if nodeKind != nil && strings.TrimSpace(nodeKind.Value) != "Pod" {
		fmt.Printf("%s:%d kind has unsupported value '%s'\n", base, nodeKind.Line, nodeKind.Value)
		errorsFound++
	}

	if errorsFound > 0 {
		os.Exit(1)
	}
}
