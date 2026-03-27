package cmd

import "raioz/internal/i18n"

// This file is named with a "zzz_" prefix to ensure its init() runs AFTER all
// other cmd/*.go init() functions. Go processes init() functions in source file
// name order within a package, so this file runs last.
//
// By the time this init() executes:
// 1. root.go init() has already called i18n.Init("") (translations are loaded)
// 2. All other cmd files have registered their flags via their own init()
// 3. All AddCommand() calls have been made
//
// We can now safely override Short/Long descriptions and flag Usage strings
// with i18n-translated values.

func init() {
	setI18nDescriptions()
}

// setI18nDescriptions overrides all command Short/Long descriptions and flag
// descriptions with i18n-translated strings.
//
// The commands themselves are initialized with plain English defaults so they
// work even if i18n initialization fails.
func setI18nDescriptions() {
	// --- Root command ---
	rootCmd.Short = i18n.T("cmd.root.short")

	// --- Root flags ---
	rootCmd.PersistentFlags().Lookup("log-level").Usage = i18n.T("flag.log_level")
	rootCmd.PersistentFlags().Lookup("log-json").Usage = i18n.T("flag.log_json")
	rootCmd.PersistentFlags().Lookup("lang").Usage = i18n.T("flag.lang")

	// --- up ---
	upCmd.Short = i18n.T("cmd.up.short")
	upCmd.Long = i18n.T("cmd.up.long")
	upCmd.Flags().Lookup("file").Usage = i18n.T("flag.up.file")
	upCmd.Flags().Lookup("profile").Usage = i18n.T("flag.profile")
	upCmd.Flags().Lookup("force-reclone").Usage = i18n.T("flag.force_reclone")
	upCmd.Flags().Lookup("dry-run").Usage = i18n.T("flag.up.dry_run")

	// --- down ---
	downCmd.Short = i18n.T("cmd.down.short")
	downCmd.Long = i18n.T("cmd.down.long")
	downCmd.Flags().Lookup("file").Usage = i18n.T("flag.down.file")
	downCmd.Flags().Lookup("project").Usage = i18n.T("flag.project")
	downCmd.Flags().Lookup("all").Usage = i18n.T("flag.down.all")
	downCmd.Flags().Lookup("prune-shared").Usage = i18n.T("flag.prune_shared")

	// --- status ---
	statusCmd.Short = i18n.T("cmd.status.short")
	statusCmd.Long = i18n.T("cmd.status.long")
	statusCmd.Flags().Lookup("file").Usage = i18n.T("flag.file")
	statusCmd.Flags().Lookup("project").Usage = i18n.T("flag.project")
	statusCmd.Flags().Lookup("json").Usage = i18n.T("flag.status.json")

	// --- logs ---
	logsCmd.Short = i18n.T("cmd.logs.short")
	logsCmd.Long = i18n.T("cmd.logs.long")
	logsCmd.Flags().Lookup("file").Usage = i18n.T("flag.file")
	logsCmd.Flags().Lookup("project").Usage = i18n.T("flag.project")
	logsCmd.Flags().Lookup("follow").Usage = i18n.T("flag.follow")
	logsCmd.Flags().Lookup("tail").Usage = i18n.T("flag.tail")
	logsCmd.Flags().Lookup("all").Usage = i18n.T("flag.logs.all")

	// --- clean ---
	cleanCmd.Short = i18n.T("cmd.clean.short")
	cleanCmd.Long = i18n.T("cmd.clean.long")
	cleanCmd.Flags().Lookup("file").Usage = i18n.T("flag.file")
	cleanCmd.Flags().Lookup("project").Usage = i18n.T("flag.project")
	cleanCmd.Flags().Lookup("all").Usage = i18n.T("flag.clean.all")
	cleanCmd.Flags().Lookup("images").Usage = i18n.T("flag.images")
	cleanCmd.Flags().Lookup("volumes").Usage = i18n.T("flag.volumes")
	cleanCmd.Flags().Lookup("networks").Usage = i18n.T("flag.networks")
	cleanCmd.Flags().Lookup("dry-run").Usage = i18n.T("flag.dry_run")
	cleanCmd.Flags().Lookup("force").Usage = i18n.T("flag.clean.force")

	// --- check ---
	checkCmd.Short = i18n.T("cmd.check.short")
	checkCmd.Long = i18n.T("cmd.check.long")
	checkCmd.Flags().Lookup("file").Usage = i18n.T("flag.file")
	checkCmd.Flags().Lookup("project").Usage = i18n.T("flag.project")

	// --- ci ---
	ciCmd.Short = i18n.T("cmd.ci.short")
	ciCmd.Long = i18n.T("cmd.ci.long")
	ciCmd.Flags().Lookup("file").Usage = i18n.T("flag.file")
	ciCmd.Flags().Lookup("keep").Usage = i18n.T("flag.ci.keep")
	ciCmd.Flags().Lookup("ephemeral").Usage = i18n.T("flag.ci.ephemeral")
	ciCmd.Flags().Lookup("job-id").Usage = i18n.T("flag.ci.job_id")
	ciCmd.Flags().Lookup("skip-build").Usage = i18n.T("flag.ci.skip_build")
	ciCmd.Flags().Lookup("skip-pull").Usage = i18n.T("flag.ci.skip_pull")
	ciCmd.Flags().Lookup("only-validate").Usage = i18n.T("flag.ci.only_validate")
	ciCmd.Flags().Lookup("force-reclone").Usage = i18n.T("flag.force_reclone")

	// --- compare ---
	compareCmd.Short = i18n.T("cmd.compare.short")
	compareCmd.Long = i18n.T("cmd.compare.long")
	compareCmd.Flags().Lookup("file").Usage = i18n.T("flag.compare.file")
	compareCmd.Flags().Lookup("production").Usage = i18n.T("flag.compare.production")
	compareCmd.Flags().Lookup("json").Usage = i18n.T("flag.compare.json")

	// --- health ---
	healthCmd.Short = i18n.T("cmd.health.short")
	healthCmd.Long = i18n.T("cmd.health.long")
	healthCmd.Flags().Lookup("file").Usage = i18n.T("flag.file")

	// --- version ---
	versionCmd.Short = i18n.T("cmd.version.short")
	versionCmd.Long = i18n.T("cmd.version.long")

	// --- init ---
	initCmd.Short = i18n.T("cmd.init.short")
	initCmd.Long = i18n.T("cmd.init.long")
	initCmd.Flags().Lookup("output").Usage = i18n.T("flag.init.output")

	// --- list ---
	listCmd.Short = i18n.T("cmd.list.short")
	listCmd.Long = i18n.T("cmd.list.long")
	listCmd.Flags().Lookup("json").Usage = i18n.T("flag.json")
	listCmd.Flags().Lookup("filter").Usage = i18n.T("flag.list.filter")
	listCmd.Flags().Lookup("status").Usage = i18n.T("flag.list.status")

	// --- migrate ---
	migrateCmd.Short = i18n.T("cmd.migrate.short")
	migrateCmd.Long = i18n.T("cmd.migrate.long")
	migrateCmd.Flags().Lookup("compose").Usage = i18n.T("flag.migrate.compose")
	migrateCmd.Flags().Lookup("output").Usage = i18n.T("flag.migrate.output")
	migrateCmd.Flags().Lookup("project").Usage = i18n.T("flag.migrate.project")
	migrateCmd.Flags().Lookup("network").Usage = i18n.T("flag.migrate.network")

	// --- override ---
	overrideCmd.Short = i18n.T("cmd.override.short")
	overrideCmd.Long = i18n.T("cmd.override.long")
	overrideCmd.Flags().Lookup("path").Usage = i18n.T("flag.override.path")
	overrideListCmd.Short = i18n.T("cmd.override.list.short")
	overrideListCmd.Long = i18n.T("cmd.override.list.long")
	overrideRemoveCmd.Short = i18n.T("cmd.override.remove.short")
	overrideRemoveCmd.Long = i18n.T("cmd.override.remove.long")

	// --- ports ---
	portsCmd.Short = i18n.T("cmd.ports.short")
	portsCmd.Flags().Lookup("project").Usage = i18n.T("flag.ports.project")

	// --- volumes ---
	volumesCmd.Short = i18n.T("cmd.volumes.short")
	volumesCmd.Long = i18n.T("cmd.volumes.long")
	volumesCmd.Flags().Lookup("file").Usage = i18n.T("flag.volumes.file")
	volumesCmd.Flags().Lookup("project").Usage = i18n.T("flag.volumes.project")
	volumesCmd.Flags().Lookup("force").Usage = i18n.T("flag.volumes.force")

	// --- workspace ---
	workspaceCmd.Short = i18n.T("cmd.workspace.short")
	workspaceCmd.Long = i18n.T("cmd.workspace.long")
	workspaceUseCmd.Short = i18n.T("cmd.workspace.use.short")
	workspaceUseCmd.Long = i18n.T("cmd.workspace.use.long")
	workspaceListCmd.Short = i18n.T("cmd.workspace.list.short")
	workspaceListCmd.Long = i18n.T("cmd.workspace.list.long")

	// --- ignore ---
	ignoreCmd.Short = i18n.T("cmd.ignore.short")
	ignoreCmd.Long = i18n.T("cmd.ignore.long")
	ignoreAddCmd.Short = i18n.T("cmd.ignore.add.short")
	ignoreAddCmd.Long = i18n.T("cmd.ignore.add.long")
	ignoreRemoveCmd.Short = i18n.T("cmd.ignore.remove.short")
	ignoreRemoveCmd.Long = i18n.T("cmd.ignore.remove.long")
	ignoreListCmd.Short = i18n.T("cmd.ignore.list.short")
	ignoreListCmd.Long = i18n.T("cmd.ignore.list.long")

	// --- link ---
	linkCmd.Short = i18n.T("cmd.link.short")
	linkCmd.Long = i18n.T("cmd.link.long")
	linkAddCmd.Short = i18n.T("cmd.link.add.short")
	linkAddCmd.Long = i18n.T("cmd.link.add.long")
	linkRemoveCmd.Short = i18n.T("cmd.link.remove.short")
	linkRemoveCmd.Long = i18n.T("cmd.link.remove.long")
	linkListCmd.Short = i18n.T("cmd.link.list.short")
	linkListCmd.Long = i18n.T("cmd.link.list.long")

	// --- lang ---
	langCmd.Short = i18n.T("cmd.lang.short")
	langSetCmd.Short = i18n.T("cmd.lang.set.short")
	langListCmd.Short = i18n.T("cmd.lang.list.short")
}
