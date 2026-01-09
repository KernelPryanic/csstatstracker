package csstatstracker

import "embed"

//go:embed migrations/*.sql
var MigrationsFS embed.FS

//go:embed sound/*.wav
var SoundFS embed.FS

//go:embed Icon.png
var IconData []byte
