# Fecha
2026-05-13

# Objetivo
Crear `gitzip`, un CLI en Go que comprime el proyecto de la carpeta actual en un archivo ZIP nombrado con la carpeta raíz, respetando reglas `.gitignore` y mostrando progreso de compresión.

# Decisiones tomadas
- Se implementó una arquitectura modular separando orquestación, compresión y progreso.
- Se usa una librería especializada para resolver `.gitignore` raíz y anidados con semántica tipo Git.
- La barra de progreso se implementó internamente para evitar dependencias visuales extra.
- Se ajustó a un formato compacto de una sola línea, sin mostrar rutas de archivos, para evitar saltos visuales en CMD/PowerShell.
- El ZIP se genera dentro de la carpeta actual y se excluye explícitamente del propio escaneo.
- La carpeta `.git/` se omite automáticamente por el walker de gitignore.
- Se preservan carpetas vacías y enlaces simbólicos.

# Arquitectura actual
- `cmd/gitzip`: punto de entrada del CLI.
- `internal/app`: control del flujo principal y salida al usuario.
- `internal/archive`: descubrimiento de entradas y creación del ZIP.
- `internal/progress`: barra de progreso compacta basada en bytes y lector contador.

# Librerías usadas
- `github.com/git-pkgs/gitignore v1.1.2` para respetar `.gitignore` anidados, negaciones y patrones avanzados.
- Biblioteca estándar de Go para ZIP, IO, filesystem y formateo de salida.

# Archivos importantes modificados
- `go.mod`
- `cmd/gitzip/main.go`
- `internal/app/app.go`
- `internal/archive/archive.go`
- `internal/archive/archive_test.go`
- `internal/progress/progress.go`
- `README.md`
- `.gitignore`

# Problemas encontrados
- Comprimir directamente dentro de la carpeta puede provocar que el ZIP previo se incluya si no se filtra.
- Implementar `.gitignore` manualmente sería propenso a errores con reglas anidadas y patrones negados.
- Los enlaces simbólicos requieren tratamiento especial para no copiar inadvertidamente el contenido del destino.
- La barra original podía extenderse demasiado al mostrar rutas largas y provocar saltos visuales en terminales estrechas.

# Soluciones implementadas
- El path del ZIP destino se filtra de forma explícita durante el escaneo.
- Se delegó el matching de `.gitignore` a una librería dedicada.
- Los symlinks se guardan como symlinks en el archivo ZIP escribiendo su target como payload.
- Se añadieron pruebas para confirmar que exclusiones raíz y anidadas se respetan.
- Se redujo la barra de progreso y se eliminó el nombre del archivo activo para mantener una sola línea.

# Pendientes
- Añadir flags opcionales como `--output`, `--quiet` o `--overwrite=false` si el flujo de uso lo requiere.
- Evaluar modo `--dry-run` para listar qué se comprimiría sin crear ZIP.
- Añadir builds multiplataforma automatizados.

# Próximos pasos
- Compilar binarios para Windows, Linux y macOS.
- Probarlo en proyectos reales de Go, Laravel y Node.
- Si se vuelve herramienta frecuente, publicar releases versionados.
