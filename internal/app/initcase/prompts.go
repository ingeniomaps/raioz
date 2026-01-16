package initcase

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// promptProjectInfo prompts the user for project name and network name
func (uc *UseCase) promptProjectInfo() (string, string, error) {
	reader := bufio.NewReader(os.Stdin)

	// Ask for project name
	projectName, err := uc.promptString(reader, "Project name", "my-project")
	if err != nil {
		return "", "", fmt.Errorf("failed to prompt project name: %w", err)
	}

	// Ask for network name (with suggestion based on project name)
	defaultNetwork := fmt.Sprintf("%s-network", projectName)
	networkName, err := uc.promptString(reader, "Docker network name", defaultNetwork)
	if err != nil {
		return "", "", fmt.Errorf("failed to prompt network name: %w", err)
	}

	return projectName, networkName, nil
}

// checkFileExists checks if the output file exists and asks for confirmation
func (uc *UseCase) checkFileExists(outputPath string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)

	if _, err := os.Stat(outputPath); err == nil {
		fmt.Printf("⚠️  File '%s' already exists. Overwrite? (y/N): ", outputPath)
		response, err := reader.ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("failed to read response: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			return false, nil
		}
	}

	return true, nil
}

// promptString prompts for a string input with a default value
func (uc *UseCase) promptString(reader *bufio.Reader, prompt string, defaultValue string) (string, error) {
	fmt.Printf("%s [%s]: ", prompt, defaultValue)
	response, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	response = strings.TrimSpace(response)
	if response == "" {
		return defaultValue, nil
	}
	return response, nil
}

// showWelcomeMessage displays the welcome message
func (uc *UseCase) showWelcomeMessage() {
	fmt.Println("🚀 Welcome to Raioz!")
	fmt.Println()
	fmt.Println("This wizard will help you create a basic .raioz.json configuration file.")
	fmt.Println("You can add services and infrastructure later by editing the file.")
	fmt.Println()
}

// showSuccessMessage displays the success message after creating the config file
func (uc *UseCase) showSuccessMessage(outputPath string) {
	fmt.Println()
	fmt.Printf("✔ Configuration file created: %s\n", outputPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review and edit .raioz.json to add your services and infrastructure")
	fmt.Println("  2. Run 'raioz up' to start your project")
	fmt.Println("  3. See 'raioz --help' for available commands")
	fmt.Println()
}
