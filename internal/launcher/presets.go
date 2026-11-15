package launcher

// TerminalPresets returns independent copies of the supported terminal commands.
func TerminalPresets() []TerminalPreset {
	return []TerminalPreset{
		{
			ID: "ptyxis", Name: "Ptyxis", Executable: "ptyxis",
			Args: []string{"--new-window", "--working-directory={path}"},
		},
		{
			ID: "gnome-terminal", Name: "GNOME Terminal", Executable: "gnome-terminal",
			Args: []string{"--window", "--working-directory={path}"},
		},
		{ID: "kitty", Name: "kitty", Executable: "kitty", Args: []string{"--directory", PathPlaceholder}},
		{
			ID: "alacritty", Name: "Alacritty", Executable: "alacritty",
			Args: []string{"--working-directory", PathPlaceholder},
		},
		{ID: "wezterm", Name: "WezTerm", Executable: "wezterm", Args: []string{"start", "--cwd", PathPlaceholder}},
		{ID: "konsole", Name: "Konsole", Executable: "konsole", Args: []string{"--workdir", PathPlaceholder}},
	}
}
