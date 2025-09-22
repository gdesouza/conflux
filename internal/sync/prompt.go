package sync

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type PromptChoice struct {
	Key         string
	Description string
	Default     bool
}

type UserChoice struct {
	Action        string // "continue", "cancel", "select"
	SelectedFiles []string
}

func PromptUser(message string, choices []PromptChoice) string {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("%s ", message)

		// Show choices
		var choiceStr []string
		var defaultChoice string
		for _, choice := range choices {
			if choice.Default {
				choiceStr = append(choiceStr, strings.ToUpper(choice.Key))
				defaultChoice = choice.Key
			} else {
				choiceStr = append(choiceStr, choice.Key)
			}
		}
		fmt.Printf("[%s]: ", strings.Join(choiceStr, "/"))

		if !scanner.Scan() {
			return "cancel"
		}

		input := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if input == "" {
			input = defaultChoice
		}

		// Validate input
		for _, choice := range choices {
			if strings.ToLower(choice.Key) == input {
				return input
			}
		}

		fmt.Println("Invalid choice. Please try again.")
	}
}

func PromptForConfirmation(summary string, changedCount, newCount, upToDateCount int) UserChoice {
	fmt.Println("\n" + summary)
	fmt.Printf("\n📊 Summary: %d changed, %d new, %d up-to-date\n", changedCount, newCount, upToDateCount)

	if changedCount == 0 && newCount == 0 {
		fmt.Println("✅ All pages are up-to-date. Nothing to sync.")
		return UserChoice{Action: "cancel"}
	}

	choices := []PromptChoice{
		{Key: "y", Description: "Yes, continue with sync", Default: true},
		{Key: "n", Description: "No, cancel sync", Default: false},
	}

	if changedCount > 0 || newCount > 0 {
		choices = append(choices, PromptChoice{Key: "s", Description: "Select specific files", Default: false})
	}

	choice := PromptUser("\nProceed with sync?", choices)

	switch choice {
	case "y":
		return UserChoice{Action: "continue"}
	case "n":
		return UserChoice{Action: "cancel"}
	case "s":
		return PromptForFileSelection()
	default:
		return UserChoice{Action: "cancel"}
	}
}

func PromptForFileSelection() UserChoice {
	fmt.Println("\n🎯 File Selection Mode")
	fmt.Println("Enter the numbers of files to sync (comma-separated), or 'a' for all, 'c' to cancel:")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return UserChoice{Action: "cancel"}
	}

	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if input == "c" {
		return UserChoice{Action: "cancel"}
	}
	if input == "a" {
		return UserChoice{Action: "continue"}
	}

	// Parse comma-separated numbers
	parts := strings.Split(input, ",")
	var indices []int
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if idx, err := strconv.Atoi(part); err == nil && idx > 0 {
			indices = append(indices, idx-1) // Convert to 0-based
		}
	}

	return UserChoice{
		Action: "select",
		// Note: The actual file paths will be populated by the caller
		// using these indices
	}
}

func DisplayFileList(pages []PageSyncInfo, showOnlyChanges bool) map[int]string {
	fmt.Println("\n📋 Files to be synced:")
	fmt.Println(strings.Repeat("─", 80))

	fileMap := make(map[int]string)
	index := 1

	for _, page := range pages {
		if showOnlyChanges && page.Status == StatusUpToDate {
			continue
		}

		// Show directory pages
		if page.IsDirectory {
			fmt.Printf("%3d. 📁 %s (directory page)\n", index, page.Title)
			fileMap[index-1] = page.FilePath
			index++
		} else {
			var icon string
			switch page.Status {
			case StatusNew:
				icon = "🆕"
			case StatusChanged:
				icon = "📝"
			case StatusUpToDate:
				icon = "✅"
			default:
				icon = "📄"
			}

			fmt.Printf("%3d. %s %s\n", index, icon, page.Title)
			fmt.Printf("     📂 %s\n", page.FilePath)
			fileMap[index-1] = page.FilePath
			index++
		}
	}

	fmt.Println(strings.Repeat("─", 80))
	return fileMap
}

func ConfirmDestructiveOperation(message string) bool {
	choices := []PromptChoice{
		{Key: "n", Description: "No, cancel", Default: true},
		{Key: "y", Description: "Yes, continue", Default: false},
	}

	fmt.Printf("⚠️  %s\n", message)
	choice := PromptUser("Are you sure?", choices)
	return choice == "y"
}
