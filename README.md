# gitzip

`gitzip` es un CLI rápido en Go para comprimir el proyecto ubicado en la carpeta actual.

## Qué hace

- Genera un ZIP con el nombre de la carpeta donde se ejecuta.
- Respeta `.gitignore` raíz y `.gitignore` anidados.
- Excluye `.git/` automáticamente.
- Evita incluir el ZIP que genera.
- Conserva carpetas vacías y enlaces simbólicos.
- Muestra una barra de progreso compacta basada en bytes procesados, pensada para mantenerse en una sola línea en terminales comunes.

## Ejemplo

Si estás dentro de:

```text
C:\Proyectos\mi-panel
```

al ejecutar `gitzip` se crea:

```text
C:\Proyectos\mi-panel\mi-panel.zip
```

## Compilar

```bash
go build -o gitzip ./cmd/gitzip
```

En Windows:

```powershell
go build -o gitzip.exe ./cmd/gitzip
```

## Ejecutar

Linux/macOS:

```bash
./gitzip
```

Windows PowerShell:

```powershell
.\gitzip.exe
```

## Salida esperada

```text
gitzip v0.1.0
Proyecto: mi-panel
Salida:   C:\Proyectos\mi-panel\mi-panel.zip
Escaneando archivos respetando .gitignore...
Incluidos: 148 archivos, 32 carpetas, 4.63 MB a procesar
[█████████████░░░░░░░░░░░]  54.22% | 2.51 MB / 4.63 MB
```

## Semántica de exclusión

`gitzip` usa reglas de exclusión compatibles con la lógica de `.gitignore`:

- Patrones normales (`*.log`, `vendor/`).
- Reglas negadas (`!archivo.txt`).
- Coincidencia con `**`.
- Reglas anidadas aplicadas a la subcarpeta correspondiente.

## Estructura

```text
cmd/gitzip/           Punto de entrada del CLI
internal/app/         Orquestación de ejecución
internal/archive/     Escaneo y creación del ZIP
internal/progress/    Barra de progreso y conteo de bytes
contexto/             Contexto técnico persistente
```
