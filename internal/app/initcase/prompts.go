package initcase

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"raioz/internal/i18n"
)

// promptProjectInfo prompts the user for project name and network name
func (uc *UseCase) promptProjectInfo() (string, string, error) {
	reader := bufio.NewReader(os.Stdin)

	// Ask for project name
	projectName, err := uc.promptString(reader, i18n.T("init.prompt_project_name"), "my-project")
	if err != nil {
		return "", "", fmt.Errorf("failed to prompt project name: %w", err)
	}

	// Ask for network name (with suggestion based on project name)
	defaultNetwork := fmt.Sprintf("%s-network", projectName)
	networkName, err := uc.promptString(reader, i18n.T("init.prompt_network_name"), defaultNetwork)
	if err != nil {
		return "", "", fmt.Errorf("failed to prompt network name: %w", err)
	}

	return projectName, networkName, nil
}

// checkFileExists checks if the output file exists and asks for confirmation
func (uc *UseCase) checkFileExists(outputPath string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)

	if _, err := os.Stat(outputPath); err == nil {
		fmt.Print(i18n.T("init.file_exists_overwrite", outputPath))
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
	fmt.Println(i18n.T("init.welcome"))
	fmt.Println()
	fmt.Println(i18n.T("init.wizard_help"))
	fmt.Println(i18n.T("init.edit_later"))
	fmt.Println()
}

// showSuccessMessage displays the success message after creating the config file
func (uc *UseCase) showSuccessMessage(outputPath string) {
	fmt.Println()
	fmt.Println(i18n.T("init.config_created", outputPath))
	fmt.Println()
	fmt.Println(i18n.T("init.next_steps"))
	fmt.Println(i18n.T("init.step_review"))
	fmt.Println(i18n.T("init.step_up"))
	fmt.Println(i18n.T("init.step_help"))
	fmt.Println()
}
