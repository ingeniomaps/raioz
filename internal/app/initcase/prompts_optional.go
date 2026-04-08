package initcase

import (
	"fmt"
	"strings"

	"raioz/internal/config"
	"raioz/internal/i18n"
)

// serviceResult holds the data collected from the service prompt
type serviceResult struct {
	Name   string
	Source config.SourceConfig
	Docker *config.DockerConfig
}

// infraPreset defines a common infrastructure template
type infraPreset struct {
	Name    string
	Image   string
	Tag     string
	Port    string
	Volume  string
}

var infraPresets = []infraPreset{
	{Name: "postgres", Image: "postgres", Tag: "15", Port: "5432:5432", Volume: "postgres-data:/var/lib/postgresql/data"},
	{Name: "redis", Image: "redis", Tag: "7", Port: "6379:6379"},
	{Name: "mysql", Image: "mysql", Tag: "8", Port: "3306:3306", Volume: "mysql-data:/var/lib/mysql"},
	{Name: "mongodb", Image: "mongo", Tag: "7", Port: "27017:27017", Volume: "mongo-data:/data/db"},
}

// promptConfirmation asks a yes/no question, default no
func (uc *UseCase) promptConfirmation(prompt string) bool {
	fmt.Fprint(uc.Out, prompt)
	response, err := uc.reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes" || response == "s" || response == "si"
}

// promptServices asks the user to add services in a loop
func (uc *UseCase) promptServices() ([]serviceResult, error) {
	if !uc.promptConfirmation(i18n.T("init.add_service")) {
		return nil, nil
	}

	var services []serviceResult
	for {
		svc, err := uc.promptOneService()
		if err != nil {
			return nil, err
		}
		services = append(services, *svc)

		if !uc.promptConfirmation(i18n.T("init.add_another_service")) {
			break
		}
	}
	return services, nil
}

func (uc *UseCase) promptOneService() (*serviceResult, error) {
	name, err := uc.promptString(i18n.T("init.service_name"), "my-service")
	if err != nil {
		return nil, err
	}

	kind, err := uc.promptString(i18n.T("init.source_kind"), "git")
	if err != nil {
		return nil, err
	}
	kind = strings.ToLower(kind)

	result := &serviceResult{Name: name}

	switch kind {
	case "image":
		if err := uc.promptServiceImage(result); err != nil {
			return nil, err
		}
	default:
		if err := uc.promptServiceGit(result, name); err != nil {
			return nil, err
		}
	}

	port, err := uc.promptString(i18n.T("init.docker_port"), "3000:3000")
	if err != nil {
		return nil, err
	}

	mode, err := uc.promptString(i18n.T("init.docker_mode"), "dev")
	if err != nil {
		return nil, err
	}

	result.Docker = &config.DockerConfig{
		Mode:  mode,
		Ports: []string{port},
	}

	return result, nil
}

func (uc *UseCase) promptServiceGit(result *serviceResult, name string) error {
	repo, err := uc.promptString(i18n.T("init.source_repo"), fmt.Sprintf("git@github.com:org/%s.git", name))
	if err != nil {
		return err
	}
	branch, err := uc.promptString(i18n.T("init.source_branch"), "main")
	if err != nil {
		return err
	}
	path, err := uc.promptString(i18n.T("init.source_path"), fmt.Sprintf("services/%s", name))
	if err != nil {
		return err
	}
	result.Source = config.SourceConfig{Kind: "git", Repo: repo, Branch: branch, Path: path}
	return nil
}

func (uc *UseCase) promptServiceImage(result *serviceResult) error {
	image, err := uc.promptString(i18n.T("init.source_image"), "org/my-service")
	if err != nil {
		return err
	}
	tag, err := uc.promptString(i18n.T("init.source_tag"), "latest")
	if err != nil {
		return err
	}
	result.Source = config.SourceConfig{Kind: "image", Image: image, Tag: tag}
	return nil
}

// promptInfra asks the user to select infrastructure components
func (uc *UseCase) promptInfra() (map[string]config.InfraEntry, error) {
	if !uc.promptConfirmation(i18n.T("init.add_infra")) {
		return nil, nil
	}

	fmt.Fprint(uc.Out, i18n.T("init.infra_select"))
	fmt.Fprint(uc.Out, " ")
	response, err := uc.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read selection: %w", err)
	}

	infra := make(map[string]config.InfraEntry)
	selections := strings.Split(strings.TrimSpace(response), ",")

	for _, sel := range selections {
		sel = strings.TrimSpace(sel)
		idx := -1
		switch sel {
		case "1":
			idx = 0
		case "2":
			idx = 1
		case "3":
			idx = 2
		case "4":
			idx = 3
		case "5":
			entry, err := uc.promptCustomInfra()
			if err != nil {
				return nil, err
			}
			if entry != nil {
				infra[entry.Name] = config.InfraEntry{Inline: &config.Infra{
					Image: entry.Image,
					Tag:   entry.Tag,
					Ports: []string{entry.Port},
				}}
			}
			continue
		default:
			continue
		}

		if idx >= 0 && idx < len(infraPresets) {
			p := infraPresets[idx]
			inf := &config.Infra{Image: p.Image, Tag: p.Tag, Ports: []string{p.Port}}
			if p.Volume != "" {
				inf.Volumes = []string{p.Volume}
			}
			infra[p.Name] = config.InfraEntry{Inline: inf}
		}
	}

	if len(infra) == 0 {
		return nil, nil
	}
	return infra, nil
}

func (uc *UseCase) promptCustomInfra() (*infraPreset, error) {
	name, err := uc.promptString(i18n.T("init.infra_custom_name"), "my-db")
	if err != nil {
		return nil, err
	}
	image, err := uc.promptString(i18n.T("init.infra_custom_image"), "postgres")
	if err != nil {
		return nil, err
	}
	tag, err := uc.promptString(i18n.T("init.infra_custom_tag"), "latest")
	if err != nil {
		return nil, err
	}
	port, err := uc.promptString(i18n.T("init.infra_custom_port"), "5432:5432")
	if err != nil {
		return nil, err
	}
	return &infraPreset{Name: name, Image: image, Tag: tag, Port: port}, nil
}
