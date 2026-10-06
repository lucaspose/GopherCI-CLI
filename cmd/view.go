package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucaspose/goci-cli/internal/api"
)

func (m model) View() string {
	switch m.screen {
	case screenLogin:
		return viewLogin(m)
	case screenMenu:
		return viewMenu(m)
	case screenRepos:
		return viewRepos(m)
	case screenJobs:
		return viewJobs(m)
	case screenJobsActions:
		return viewJobsActions(m)
	case screenLogs:
		return viewLogs(m)
	case screenSSHKeys:
		return viewSSHKeys(m)
	case screenNewJob:
		return viewNewJob(m)
	case screenSettings:
		return viewSettings(m)
	case screenHelp:
		return viewHelp(m)
	}
	return ""
}

// jobDuration retourne une chaîne lisible comme "2m34s" ou "5s".
// Pour un job en cours, le temps est calculé depuis CreatedAt jusqu'à now.
func jobDuration(job api.Job) string {
	end := time.Now()
	if job.FinishedAt != nil {
		end = *job.FinishedAt
	}
	d := end.Sub(job.CreatedAt).Round(time.Second)
	if d < 0 {
		return ""
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// ── Login ────────────────────────────────────────────────────────────────────

func viewLogin(m model) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + brandStyle.Render("GopherCI") + "\n")
	b.WriteString("  " + dimStyle.Render("Jenkins simplifié • CI/CD en ligne de commande") + "\n\n")

	options := []string{"Connexion par email", "Connexion via GitHub"}
	for i, opt := range options {
		b.WriteString("  " + renderItem(opt, i == m.login.cursor) + "\n")
	}

	if m.login.cursor == 0 {
		b.WriteString("\n")
		b.WriteString("  " + dimStyle.Render("Email") + "\n")
		b.WriteString("  " + m.login.inputs[0].View() + "\n\n")
		b.WriteString("  " + dimStyle.Render("Mot de passe") + "\n")
		b.WriteString("  " + m.login.inputs[1].View() + "\n")
	}

	b.WriteString(renderFeedback(m))
	b.WriteString("\n" + renderSep() + "\n")
	hints := []string{"↑↓ naviguer", "enter sélectionner", "tab changer de champ"}
	if m.githubAuthFailed {
		hints = append(hints, "r réessayer GitHub")
	}
	b.WriteString(renderHelp(hints...) + "\n")
	return b.String()
}

// ── Menu ─────────────────────────────────────────────────────────────────────

func viewMenu(m model) string {
	var b strings.Builder
	b.WriteString("\n")
	w := m.width
	if w == 0 {
		w = 80
	}
	b.WriteString(renderWelcomePanel(w, m.config.APIURL) + "\n\n")

	items := []string{"Repositories", "SSH Keys", "Settings", "Help", "Logout"}
	for i, item := range items {
		if i == 2 {
			b.WriteString("  " + dimStyle.Render("  ─────────────────") + "\n")
		}
		b.WriteString("  " + renderItem(item, i == m.menuCursor) + "\n")
	}

	b.WriteString(renderFeedback(m))
	b.WriteString("\n" + renderSep() + "\n")
	b.WriteString(renderHelp("↑↓ naviguer", "enter sélectionner", "ctrl+c quitter") + "\n")
	return b.String()
}

// renderWelcomePanel builds the styled box at the top of the menu.
// In lipgloss v1.x Width() is the total outer size (border + padding + content).
func renderWelcomePanel(w int, apiURL string) string {
	// Keep a small safety margin to avoid clipping the top-right rounded corner.
	boxW := w - 4
	if boxW < 28 {
		boxW = 28
	}
	// inner content width = boxW - border(2) - padding horizontal(4)
	innerW := boxW - 6
	if innerW < 20 {
		innerW = 20
	}

	// Center the mascot manually – lipgloss.Width handles Unicode visual width.
	const mascotW = 14
	leftPad := (innerW - mascotW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	pad := strings.Repeat(" ", leftPad)
	lines := strings.Split(mascotASCII, "\n")
	centeredLines := make([]string, len(lines))
	for i, l := range lines {
		centeredLines[i] = pad + l
	}
	mascotBlock := lipgloss.NewStyle().Foreground(colorMascot).Render(
		strings.Join(centeredLines, "\n"))

	center := lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center)

	title := center.Bold(true).Foreground(colorBrand).Render("GopherCI")
	subtitle := center.Foreground(colorDim).Render("CI/CD simplifié • Jenkins en ligne de commande")

	serverLabel := "⬡ " + apiURL
	if apiURL == "" {
		serverLabel = "⬡ serveur non configuré"
	}
	serverLine := center.Foreground(colorDim).Render(serverLabel)

	content := mascotBlock + "\n" + title + "\n" + subtitle + "\n" + serverLine

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBrand).
		Padding(1, 2).
		Width(boxW).
		Render(content)
}

// ── Repositories ─────────────────────────────────────────────────────────────

func viewRepos(m model) string {
	if m.loading {
		return "\n" + lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(m.spinner.View()+" "+m.loadingMsg) + "\n"
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(renderBreadcrumb("Menu", "Repositories") + "\n\n")
	pageSize := m.reposPageSize()

	// GitHub flow
	if len(m.repos.githubRepos) > 0 {
		start, end, totalPages, currentPage, _ := githubRepoPageBounds(
			m.repos.githubRepos,
			m.githubReposRowBudget(),
			m.repos.githubPageOffset,
		)

		b.WriteString("  " + headerStyle.Render("Dépôts GitHub") + "\n")
		b.WriteString(renderSepWidth(m.width) + "\n")
		currentOwner := ""
		for i := start; i < end; i++ {
			repo := m.repos.githubRepos[i]
			owner := githubRepoOwner(repo.FullName)
			if owner != currentOwner {
				currentOwner = owner
				b.WriteString("  " + dimStyle.Render(owner+"/") + "\n")
			}
			selected := i == m.repos.githubCursor
			label := repo.Name
			if repo.Private {
				label += "  " + monoStyle.Render("privé")
			}
			b.WriteString("  " + renderItem(label, selected) + "\n")
		}
		if m.repos.selectedGH != nil {
			b.WriteString("\n  " + successStyle.Render("✔  Sélectionné : "+m.repos.selectedGH.FullName) + "\n")
		}
		if totalPages > 1 {
			b.WriteString("\n" + renderPaginationWithHint(currentPage, totalPages, "←/→ changer de page") + "\n")
		}
		b.WriteString(renderFeedback(m))
		b.WriteString("\n" + renderSepWidth(m.width) + "\n")
		hints := []string{"↑↓ naviguer", "enter sélectionner"}
		hints = append(hints, "j ouvrir les jobs", "esc retour")
		b.WriteString(renderHelp(hints...) + "\n")
		return b.String()
	}

	// Org flow
	if len(m.repos.orgs) == 0 && len(m.repos.repos) == 0 {
		b.WriteString(renderEmpty("Aucune organisation trouvée", "Vérifiez votre connexion ou créez une organisation."))
		b.WriteString("\n" + renderSepWidth(m.width) + "\n")
		b.WriteString(renderHelp("esc retour") + "\n")
		return b.String()
	}

	if m.repos.subMode == 0 {
		start, end, totalPages, offset := paginateRange(len(m.repos.orgs), pageSize, m.repos.orgPageOffset)
		currentPage := 1
		if pageSize > 0 {
			currentPage = offset/pageSize + 1
		}

		b.WriteString("  " + headerStyle.Render("Organisations") + "\n")
		b.WriteString(renderSepWidth(m.width) + "\n")
		for i := start; i < end; i++ {
			org := m.repos.orgs[i]
			b.WriteString("  " + renderItem(org.Name, i == m.repos.orgCursor) + "\n")
		}

		if totalPages > 1 {
			b.WriteString("\n" + renderPaginationWithHint(currentPage, totalPages, "←/→ changer de page") + "\n")
		}

		b.WriteString(renderFeedback(m))
		b.WriteString("\n" + renderSepWidth(m.width) + "\n")
		hints := []string{"↑↓ naviguer", "enter sélectionner"}
		hints = append(hints, "esc retour")
		b.WriteString(renderHelp(hints...) + "\n")
		return b.String()
	}

	title := "Dépôts"
	if m.repos.selectedOrg != nil {
		title += " — " + m.repos.selectedOrg.Name
	}

	b.WriteString("  " + headerStyle.Render(title) + "\n")
	b.WriteString(renderSepWidth(m.width) + "\n")

	start, end, totalPages, offset := paginateRange(len(m.repos.repos), pageSize, m.repos.repoPageOffset)
	currentPage := 1
	if pageSize > 0 {
		currentPage = offset/pageSize + 1
	}

	if len(m.repos.repos) == 0 {
		b.WriteString(renderEmpty("Aucun dépôt", ""))
	} else {
		for i := start; i < end; i++ {
			repo := m.repos.repos[i]
			b.WriteString("  " + renderItem(repo.Name, i == m.repos.repoCursor) + "\n")
		}
	}

	if m.repos.selectedRepo != nil {
		b.WriteString("\n  " + successStyle.Render("✔  Sélectionné : "+m.repos.selectedRepo.Name) + "\n")
	}

	if totalPages > 1 {
		b.WriteString("\n" + renderPaginationWithHint(currentPage, totalPages, "←/→ changer de page") + "\n")
	}

	b.WriteString(renderFeedback(m))
	b.WriteString("\n" + renderSepWidth(m.width) + "\n")
	hints := []string{"↑↓ naviguer", "enter sélectionner"}
	if m.hasRepoContext() {
		hints = append(hints, "j ouvrir les jobs")
	}
	hints = append(hints, "esc retour")
	b.WriteString(renderHelp(hints...) + "\n")
	return b.String()
}

// ── Jobs ─────────────────────────────────────────────────────────────────────

func viewJobs(m model) string {
	if m.loading {
		return "\n" + lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(m.spinner.View()+" "+m.loadingMsg) + "\n"
	}

	var b strings.Builder
	b.WriteString("\n")

	// Breadcrumb
	crumbs := []string{"Menu"}
	if repo := repoLabel(m); repo != "" {
		crumbs = append(crumbs, repo)
	}
	crumbs = append(crumbs, "Jobs")
	b.WriteString(renderBreadcrumb(crumbs...) + "\n\n")

	// Header with optional filter badge
	header := "Jobs"
	if m.jobs.filter != "" {
		header += "  " + warnStyle.Render("["+m.jobs.filter+"]")
	}
	b.WriteString("  " + headerStyle.Render(header) + "\n")
	b.WriteString(renderSep() + "\n")

	visible := filterJobs(m.jobs.jobs, m.jobs.filter)
	page, totalPages := paginateJobs(visible, m.jobs.pageSize, m.jobs.pageOffset)

	if len(visible) == 0 {
		if m.jobs.filter != "" {
			b.WriteString(renderEmpty("Aucun job avec le statut \""+m.jobs.filter+"\"", "Appuyez sur f pour changer le filtre."))
		} else {
			b.WriteString(renderEmpty("Aucun job trouvé", "Appuyez sur n pour créer un job."))
		}
	} else {
		currentPage := m.jobs.pageOffset/m.jobs.pageSize + 1
		pageIdx := m.jobs.cursor - m.jobs.pageOffset
		for i, job := range page {
			selected := i == pageIdx
			id := dimStyle.Render(shortID(job.ID))
			st := dimStyle.Render(job.Status)
			if selected {
				id = selectedTextStyle.Render(shortID(job.ID))
				st = statusText(job.Status)
			}
			ts := dimStyle.Render(job.CreatedAt.Format("02/01 15:04"))
			dur := dimStyle.Render(jobDuration(job))
			b.WriteString(fmt.Sprintf("  %s %s  %s  %s  %s  %s\n",
				cursorIf(selected), statusDot(job.Status), id, st, ts, dur))
		}
		if totalPages > 1 {
			b.WriteString("\n" + renderPagination(currentPage, totalPages) + "\n")
		}
	}

	b.WriteString(renderFeedback(m))
	b.WriteString("\n" + renderSep() + "\n")
	b.WriteString(renderHelp("↑↓ naviguer", "enter ouvrir", "n nouveau", "f filtrer", "esc retour") + "\n")
	return b.String()
}

// ── Job actions ──────────────────────────────────────────────────────────────

func viewJobsActions(m model) string {
	if m.selectedJob == nil {
		return "\n  " + dimStyle.Render("Aucun job sélectionné") + "\n\n" +
			renderSep() + "\n" + renderHelp("esc retour") + "\n"
	}

	var b strings.Builder
	b.WriteString("\n")

	crumbs := []string{"Menu"}
	if repo := repoLabel(m); repo != "" {
		crumbs = append(crumbs, repo)
	}
	crumbs = append(crumbs, "Jobs", "Job "+shortID(m.selectedJob.ID))
	b.WriteString(renderBreadcrumb(crumbs...) + "\n\n")

	b.WriteString("  " + headerStyle.Render("Job "+shortID(m.selectedJob.ID)) + "\n")
	b.WriteString(fmt.Sprintf("  %s  %s  %s\n", statusDot(m.selectedJob.Status), statusText(m.selectedJob.Status), dimStyle.Render(jobDuration(*m.selectedJob))))
	b.WriteString("  " + dimStyle.Render("Créé le  "+m.selectedJob.CreatedAt.Format("2006-01-02 15:04:05")) + "\n\n")

	if m.actions.confirmDelete {
		b.WriteString(renderConfirm("Supprimer ce job définitivement ?", "y confirmer", "esc annuler") + "\n")
	} else {
		actions := []string{"Voir les logs", "Re-lancer", "Télécharger le ZIP", "Supprimer"}
		for i, action := range actions {
			b.WriteString("  " + renderItem(action, i == m.actions.cursor) + "\n")
		}
	}

	b.WriteString(renderFeedback(m))
	b.WriteString("\n" + renderSep() + "\n")
	if m.actions.confirmDelete {
		b.WriteString(renderHelp("y confirmer", "esc annuler") + "\n")
	} else {
		b.WriteString(renderHelp("↑↓ naviguer", "enter confirmer", "esc retour") + "\n")
	}
	return b.String()
}

// ── Logs ─────────────────────────────────────────────────────────────────────

func viewLogs(m model) string {
	if m.selectedJob == nil {
		return "\n  " + dimStyle.Render("Aucun job sélectionné") + "\n\n" +
			renderSep() + "\n" + renderHelp("esc retour") + "\n"
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n\n",
		headerStyle.Render("Logs"),
		dimStyle.Render(shortID(m.selectedJob.ID)),
		statusDot(m.selectedJob.Status),
		statusText(m.selectedJob.Status)))

	logs := m.selectedJob.Logs
	if len(logs) == 0 {
		b.WriteString("  " + dimStyle.Render("Aucun log disponible") + "\n")
	} else {
		pageSize := m.logsPageSize()
		offset := clamp(m.logs.offset, 0, len(logs)-1)
		end := offset + pageSize
		if end > len(logs) {
			end = len(logs)
		}
		for _, line := range logs[offset:end] {
			b.WriteString(line + "\n")
		}
		// Indicateur de position
		if len(logs) > pageSize {
			indicator := fmt.Sprintf("ligne %d-%d / %d", offset+1, end, len(logs))
			b.WriteString("\n  " + dimStyle.Render(indicator) + "\n")
		}
	}

	b.WriteString("\n" + renderSep() + "\n")
	b.WriteString(renderHelp("↑↓ scroller", "pgup/pgdn page", "esc retour") + "\n")
	return b.String()
}

// ── New Job ───────────────────────────────────────────────────────────────────

func viewNewJob(m model) string {
	if m.loading {
		return "\n" + lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(m.spinner.View()+" "+m.loadingMsg) + "\n"
	}

	var b strings.Builder
	b.WriteString("\n")

	crumbs := []string{"Menu"}
	if repo := repoLabel(m); repo != "" {
		crumbs = append(crumbs, repo)
	}
	crumbs = append(crumbs, "Jobs", "Nouveau Job")
	b.WriteString(renderBreadcrumb(crumbs...) + "\n\n")

	b.WriteString("  " + headerStyle.Render("Nouveau Job") + "\n")
	b.WriteString(renderSep() + "\n\n")

	repo := repoLabel(m)
	if repo != "" {
		b.WriteString("  " + dimStyle.Render("Dépôt  ") + dimStyle.Render(repo) + "\n\n")
	}

	switch m.newJob.mode {
	case 0:
		if m.newJob.hasGoci {
			label := "Depuis .goci"
			if m.newJob.pipeline != "" {
				label += "  " + dimStyle.Render(m.newJob.pipeline)
			}
			if len(m.newJob.steps) > 0 {
				names := make([]string, len(m.newJob.steps))
				for i, s := range m.newJob.steps {
					names[i] = s.Name
				}
				label += "  " + dimStyle.Render(strings.Join(names, " · "))
			}
			b.WriteString("  " + renderItem(label, m.newJob.cursor == 0) + "\n")
		} else {
			b.WriteString("  " + dimStyle.Render("  Depuis .goci  ") +
				dimStyle.Render("(aucun fichier .goci dans le répertoire courant)") + "\n")
		}
		b.WriteString("  " + renderItem("Commande rapide", m.newJob.cursor == 1) + "\n")

		b.WriteString(renderFeedback(m))
		b.WriteString("\n" + renderSep() + "\n")
		b.WriteString(renderHelp("↑↓ naviguer", "enter sélectionner", "esc retour") + "\n")

	case 1:
		b.WriteString("  " + dimStyle.Render("Commande") + "\n")
		b.WriteString("  " + m.newJob.cmdInput.View() + "\n")
		b.WriteString(renderFeedback(m))
		b.WriteString("\n" + renderSep() + "\n")
		b.WriteString(renderHelp("enter lancer", "esc retour") + "\n")
	}

	return b.String()
}

// ── SSH Keys ─────────────────────────────────────────────────────────────────

func viewSSHKeys(m model) string {
	if m.loading {
		return "\n" + lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(m.spinner.View()+" "+m.loadingMsg) + "\n"
	}

	var b strings.Builder
	b.WriteString("\n")

	switch m.ssh.mode {
	case 0:
		b.WriteString(renderBreadcrumb("Menu", "SSH Keys") + "\n\n")
		b.WriteString("  " + headerStyle.Render("SSH Keys") + "\n")
		b.WriteString(renderSep() + "\n")

		if len(m.ssh.keys) == 0 {
			b.WriteString(renderEmpty("Aucune clé SSH enregistrée", "Appuyez sur a pour ajouter une clé."))
		} else {
			for i, key := range m.ssh.keys {
				label := key.Name
				if key.Fingerprint != "" {
					label += "  " + monoStyle.Render(truncate(key.Fingerprint, 40))
				} else if len(key.PublicKey) > 20 {
					parts := strings.Fields(key.PublicKey)
					if len(parts) >= 2 {
						label += "  " + monoStyle.Render(parts[0]+" "+truncate(parts[1], 16))
					}
				}
				b.WriteString("  " + renderItem(label, i == m.ssh.cursor) + "\n")
			}
		}

		if m.ssh.confirmDelete && len(m.ssh.keys) > 0 {
			key := m.ssh.keys[m.ssh.cursor]
			b.WriteString("\n" + renderConfirm("Supprimer \""+key.Name+"\" définitivement ?", "y confirmer", "esc annuler") + "\n")
		}

		b.WriteString(renderFeedback(m))
		b.WriteString("\n" + renderSep() + "\n")
		if m.ssh.confirmDelete {
			b.WriteString(renderHelp("y confirmer", "esc annuler") + "\n")
		} else {
			b.WriteString(renderHelp("↑↓ naviguer", "a ajouter", "d supprimer", "c copier fingerprint", "esc retour") + "\n")
		}

	case 1:
		b.WriteString(renderBreadcrumb("Menu", "SSH Keys", "Ajouter") + "\n\n")
		b.WriteString("  " + headerStyle.Render("Ajouter une clé SSH") + "\n")
		b.WriteString(renderSep() + "\n\n")
		b.WriteString("  " + dimStyle.Render("Nom de la clé") + "\n")
		b.WriteString("  " + m.ssh.nameInput.View() + "\n")
		b.WriteString("\n  " + dimStyle.Render("Un nom court, ex: deploy-key ou home-laptop") + "\n")
		b.WriteString(renderFeedback(m))
		b.WriteString("\n" + renderSep() + "\n")
		b.WriteString(renderHelp("enter suivant", "esc annuler") + "\n")

	case 2:
		b.WriteString(renderBreadcrumb("Menu", "SSH Keys", "Ajouter") + "\n\n")
		b.WriteString("  " + headerStyle.Render("Ajouter une clé SSH") + "\n")
		b.WriteString("  " + dimStyle.Render("nom  "+m.ssh.newName) + "\n")
		b.WriteString(renderSep() + "\n\n")
		b.WriteString("  " + dimStyle.Render("Clé privée (ou chemin)") + "\n")
		b.WriteString("  " + m.ssh.pubInput.View() + "\n")
		b.WriteString("\n  " + dimStyle.Render("Collez le contenu de ~/.ssh/id_ed25519 (sans .pub)") + "\n")
		b.WriteString("  " + dimStyle.Render("Ou entrez un chemin: ~/.ssh/id_ed25519") + "\n")
		b.WriteString("  " + dimStyle.Render("Le serveur utilisera cette clé privée pour cloner vos dépôts") + "\n")
		b.WriteString(renderFeedback(m))
		b.WriteString("\n" + renderSep() + "\n")
		b.WriteString(renderHelp("enter sauvegarder", "esc retour") + "\n")
	}

	return b.String()
}

// ── Settings ─────────────────────────────────────────────────────────────────

func viewSettings(m model) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(renderBreadcrumb("Menu", "Settings") + "\n\n")
	b.WriteString("  " + headerStyle.Render("Settings") + "\n")
	b.WriteString(renderSep() + "\n\n")

	b.WriteString("  " + dimStyle.Render("API URL") + "\n")
	if m.settings.editing {
		b.WriteString("  " + m.settings.apiURLInput.View() + "\n")
	} else {
		b.WriteString("  " + selectedTextStyle.Render(m.config.APIURL) +
			"  " + dimStyle.Render("[e pour modifier]") + "\n")
	}

	b.WriteString("\n  " + dimStyle.Render("Token") + "\n")
	if m.config.Token != "" {
		b.WriteString("  " + monoStyle.Render(truncate(m.config.Token, 24)+"…") + "\n")
	} else {
		b.WriteString("  " + dimStyle.Render("non connecté") + "\n")
	}

	if m.config.GitHubToken != "" {
		b.WriteString("\n  " + dimStyle.Render("GitHub") + "\n")
		b.WriteString("  " + successStyle.Render("Connecté") +
			"  " + monoStyle.Render(truncate(m.config.GitHubToken, 20)+"…") + "\n")
	}

	b.WriteString(renderFeedback(m))
	b.WriteString("\n" + renderSep() + "\n")
	if m.settings.editing {
		b.WriteString(renderHelp("enter sauvegarder", "esc annuler") + "\n")
	} else {
		b.WriteString(renderHelp("e modifier l'URL", "esc retour") + "\n")
	}
	return b.String()
}

// ── Help ─────────────────────────────────────────────────────────────────────

func viewHelp(m model) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(renderBreadcrumb("Menu", "Aide") + "\n\n")
	b.WriteString("  " + headerStyle.Render("Raccourcis clavier") + "\n")
	b.WriteString(renderSep() + "\n\n")

	sections := []struct {
		title string
		rows  [][2]string
	}{
		{
			"Navigation globale",
			[][2]string{
				{"↑ / ↓", "Déplacer le curseur"},
				{"enter", "Sélectionner / confirmer"},
				{"esc", "Retour / annuler"},
				{"ctrl+c", "Quitter"},
			},
		},
		{
			"Jobs",
			[][2]string{
				{"n", "Nouveau job"},
				{"f", "Cycle du filtre statut (running/failed/success/pending)"},
				{"pgdn / ctrl+f", "Page suivante"},
				{"pgup / ctrl+b", "Page précédente"},
				{"enter", "Ouvrir les actions du job"},
			},
		},
		{
			"SSH Keys",
			[][2]string{
				{"a", "Ajouter une clé"},
				{"d", "Supprimer la clé sélectionnée (avec confirmation)"},
				{"c", "Copier le fingerprint dans le presse-papiers"},
			},
		},
		{
			"Repositories",
			[][2]string{
				{"enter", "Sélectionner organisation / dépôt"},
				{"← / →", "Changer de page"},
				{"j", "Ouvrir les jobs du dépôt sélectionné"},
			},
		},
		{
			"Settings",
			[][2]string{
				{"e", "Modifier l'URL de l'API"},
			},
		},
	}

	for _, sec := range sections {
		b.WriteString("  " + warnStyle.Render(sec.title) + "\n")
		for _, row := range sec.rows {
			key := fmt.Sprintf("%-16s", row[0])
			b.WriteString("  " + dimStyle.Render(key) + "  " + dimStyle.Render(row[1]) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(renderSep() + "\n")
	b.WriteString(renderHelp("esc retour") + "\n")
	return b.String()
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// renderFeedback renders error or success messages (mutually exclusive).
func renderFeedback(m model) string {
	if m.err != "" {
		return "\n  " + errorStyle.Render("✖  "+m.err) + "\n"
	}
	if m.successMsg != "" {
		return "\n  " + successStyle.Render("✔  "+m.successMsg) + "\n"
	}
	return ""
}

// cursorIf returns the cursor glyph when selected, else spaces.
func cursorIf(selected bool) string {
	if selected {
		return cursorGlyph
	}
	return " "
}

// repoLabel returns a short display label for the currently selected repo.
func repoLabel(m model) string {
	if m.repos.selectedGH != nil {
		return m.repos.selectedGH.FullName
	}
	if m.repos.selectedRepo != nil {
		return m.repos.selectedRepo.Name
	}
	return ""
}
