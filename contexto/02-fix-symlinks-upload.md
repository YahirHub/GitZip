# Fecha
2026-05-20

# Objetivo
Corregir los errores de compresión al encontrar enlaces simbólicos, *junctions* o *reparse points* en proyectos reales como Laravel, y añadir un comando `upload` que comprima con contraseña aleatoria, suba el ZIP a hosts temporales con fallback y muestre comandos listos para descargar y descomprimir.

# Decisiones tomadas
- Se cambió el análisis de entradas a `os.Lstat` para inspeccionar el objeto del filesystem sin seguir su destino.
- Los symlinks reales se conservan como symlinks dentro del ZIP, manteniendo su target como payload.
- Las entradas especiales que no pueden serializarse de forma segura como archivo ZIP regular se omiten con aviso, en lugar de romper toda la compresión.
- Se añadió el comando `gitzip upload` como subcomando simple para mantener la UX actual de `gitzip` sin flags obligatorios.
- La contraseña se genera con `crypto/rand` y se imprime únicamente al final del flujo de subida.
- El modo `upload` usa cifrado ZIP estándar para conservar compatibilidad con `unzip -P`.
- La subida temporal usa una cadena de proveedores con fallback. Si un host falla, se intenta el siguiente.
- Se valida la existencia del ZIP antes de comenzar los intentos de subida para evitar reportes repetitivos e inútiles.

# Arquitectura actual
- `cmd/gitzip`: punto de entrada del CLI.
- `internal/app`: control del flujo principal, parsing de comandos, generación de contraseña y salida al usuario.
- `internal/archive`: descubrimiento de entradas, clasificación segura de filesystem y creación del ZIP normal o protegido.
- `internal/progress`: barra de progreso compacta basada en bytes y lector contador.
- `internal/upload`: subida multipart, parsing de respuestas, fallback entre proveedores y validación de URLs.
- `third_party/yeka_zip`: fork vendorizado mínimo para escritura/lectura de ZIP con cifrado estándar compatible con `unzip -P`.

# Librerías usadas
- `github.com/git-pkgs/gitignore v1.1.2` para respetar `.gitignore` anidados, negaciones y patrones avanzados.
- `github.com/yeka/zip` en una copia vendorizada reducida para soporte de ZIP con contraseña estándar.
- Biblioteca estándar de Go para filesystem, IO, HTTP multipart, JSON, generación criptográfica de contraseña y formateo de salida.

# Archivos importantes modificados
- `go.mod`
- `cmd/gitzip/main.go`
- `internal/app/app.go`
- `internal/archive/archive.go`
- `internal/archive/archive_test.go`
- `internal/upload/upload.go`
- `internal/upload/upload_test.go`
- `third_party/yeka_zip/*`
- `README.md`
- `contexto/02-fix-symlinks-upload.md`

# Problemas encontrados
- El flujo anterior trataba cualquier entrada que no fuera directorio o symlink como archivo regular, por lo que podía intentar abrir *junctions*, *reparse points*, sockets, FIFOs o dispositivos como si fueran archivos normales.
- En Go 1.23, Windows cambió la forma en que reporta algunos *reparse points*: ciertos mount points dejan de marcarse como `ModeSymlink` y otros pasan a `ModeIrregular`.
- Esto puede aparecer en proyectos Laravel al llegar a `public/storage`, haciendo que la compresión falle dependiendo de cómo esté creado ese enlace en Windows.
- La subida temporal necesitaba manejar respuestas distintas entre proveedores: texto plano, JSON y errores HTTP.
- Sin validación previa del archivo local, un ZIP inexistente produciría fallos repetidos contra todos los proveedores.

# Soluciones implementadas
- `Collect` ahora usa `os.Lstat` y clasifica explícitamente archivos regulares, directorios, symlinks y tipos especiales.
- Los symlinks reales se conservan correctamente en el ZIP sin seguir su destino.
- Los tipos especiales no preservables se guardan en `Stats.Skipped`, se muestran al usuario y no detienen el proceso.
- `gitzip upload` crea ZIP con contraseña aleatoria, ejecuta la subida temporal y entrega comandos `wget` y `unzip -P` listos.
- Se implementaron proveedores con fallback: `temp.sh`, `Litterbox`, `file.io`, `tmpfiles.org` y `0x0.st`.
- Se normalizan y validan las URLs de descarga antes de reportarlas al usuario.
- Se añadió una validación temprana de ruta para asegurar que la subida parta de un ZIP regular existente.
- Se añadieron pruebas para:
  - `.gitignore` raíz y anidados,
  - symlink de directorio tipo Laravel,
  - ZIP con contraseña,
  - fallback de proveedores,
  - parser de respuesta de `file.io`,
  - subida multipart básica,
  - rechazo temprano de ZIP inexistente.

# Pendientes
- Probar en un entorno Windows real con `public/storage` creado como symlink y como junction para confirmar el mensaje mostrado en cada variante.
- Evaluar si conviene agregar soporte específico para serializar junctions de Windows en lugar de omitirlos.
- Considerar flags futuros como `--output`, `--provider`, `--no-upload-log` o expiración configurable.
- Evaluar cifrado AES en un modo separado si se desea seguridad más fuerte, aunque ya no sería compatible directamente con `unzip -P`.

# Próximos pasos
- Compilar y publicar binarios para Windows, Linux y macOS.
- Probar `gitzip upload` con proyectos reales y tamaños variados para observar qué proveedor responde mejor.
- Documentar releases y mantener la lista de proveedores revisable, porque estos servicios pueden cambiar sus APIs o límites.
