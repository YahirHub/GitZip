# gitzip

`gitzip` es un CLI rápido en Go para comprimir el proyecto ubicado en la carpeta actual.

## Qué hace

- Genera un ZIP con el nombre de la carpeta donde se ejecuta.
- Respeta `.gitignore` raíz y `.gitignore` anidados.
- Excluye `.git/` automáticamente.
- Evita incluir el ZIP que genera.
- Conserva carpetas vacías y enlaces simbólicos reales.
- Tolera entradas especiales que no pueden preservarse como archivo ZIP seguro, por ejemplo algunos *reparse points* o *junctions* de Windows: los omite, los reporta y continúa la compresión en lugar de romperse.
- Muestra una barra de progreso compacta basada en bytes procesados, pensada para mantenerse en una sola línea en terminales comunes.
- Incluye el comando `upload`, que crea un ZIP protegido con contraseña aleatoria, intenta subirlo a varios hosts temporales y deja listos los comandos `wget` y `unzip -P`.

## Ejemplo de compresión normal

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

## Modo `upload`

```bash
./gitzip upload
```

En Windows PowerShell:

```powershell
.\gitzip.exe upload
```

Este modo:

1. Comprime el proyecto actual.
2. Genera una contraseña aleatoria.
3. Protege el ZIP con cifrado ZIP estándar compatible con `unzip -P`.
4. Intenta subirlo, en orden, a proveedores que entregan o permiten construir una URL de descarga directa compatible con `wget`:
   - `Litterbox`
   - `Uguu`
   - `transfer.sh`
   - `0x0.st`
5. Si un proveedor falla, prueba el siguiente.
6. Si la subida termina bien, imprime:
   - enlace directo de descarga,
   - contraseña,
   - comando `wget`,
   - comando `unzip -P`.

Ejemplo de salida:

```text
Subida temporal completada con: transfer.sh
Enlace directo: https://transfer.sh/get/abc123/mi-panel.zip
Contraseña ZIP: XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
Comando wget:
wget -O 'mi-panel.zip' 'https://transfer.sh/get/abc123/mi-panel.zip'
Comando unzip:
unzip -P 'XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX' 'mi-panel.zip'
```

> Nota: el modo `upload` usa cifrado ZIP estándar para mantener compatibilidad directa con `unzip -P`. Es útil para intercambio temporal, pero no debe tratarse como cifrado moderno de alta seguridad.

### Enlaces directos del modo `upload`

El comando ya no usa hosts que puedan responder con una página HTML de aterrizaje en lugar del archivo. La URL que imprime está pensada para que el `wget -O ...` descargue el ZIP, no una página web. En `transfer.sh`, por ejemplo, se transforma el enlace compartible al alias directo `/get/...` antes de mostrarlo.


## Salida esperada

```text
gitzip v0.2.1
Proyecto: mi-panel
Salida:   C:\Proyectos\mi-panel\mi-panel.zip
Escaneando archivos respetando .gitignore...
Incluidos: 148 archivos, 32 carpetas, 4.63 MB a procesar
[█████████████░░░░░░░░░░░]  54.22% | 2.51 MB / 4.63 MB
```

Cuando detecta una entrada especial no preservable, avisa sin cancelar todo el proceso:

```text
Omitidos por tipo especial no preservable: 1
- public/storage (entrada irregular o reparse point no preservable como archivo ZIP)
```

## Symlinks, Laravel y Windows

- Los symlinks reales se guardan como symlinks en el ZIP; no se copia el contenido de su destino.
- Un enlace típico de Laravel como `public/storage` funciona correctamente si el sistema lo expone como symlink real.
- En Windows, algunos enlaces de directorio pueden presentarse como *junctions* o *reparse points* no tratables como symlink normal. `gitzip` ahora los omite de forma segura y continúa la compresión, evitando que la creación del ZIP falle.

## Semántica de exclusión

`gitzip` usa reglas de exclusión compatibles con la lógica de `.gitignore`:

- Patrones normales (`*.log`, `vendor/`).
- Reglas negadas (`!archivo.txt`).
- Coincidencia con `**`.
- Reglas anidadas aplicadas a la subcarpeta correspondiente.

## Comandos disponibles

```text
gitzip          Comprime el proyecto actual
gitzip upload   Comprime con contraseña aleatoria y sube a un host temporal
gitzip help     Muestra la ayuda
```

## Estructura

```text
cmd/gitzip/           Punto de entrada del CLI
internal/app/         Orquestación de ejecución y comandos
internal/archive/     Escaneo y creación del ZIP
internal/progress/    Barra de progreso y conteo de bytes
internal/upload/      Subida temporal con fallback entre proveedores
contexto/             Contexto técnico persistente
third_party/          Dependencias vendorizadas
```
