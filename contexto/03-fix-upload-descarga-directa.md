# Fecha
2026-05-20

# Objetivo
Corregir el subcomando `gitzip upload` para que el enlace que imprime sea realmente compatible con descarga directa por `wget`, evitando proveedores que devuelven páginas HTML de aterrizaje en lugar del archivo ZIP.

# Problema detectado
- Un enlace retornado por `temp.sh` se usó con `wget`, pero el contenido descargado fue una página HTML y no el ZIP.
- El comando generado por `gitzip upload` asumía que todos los proveedores devolvían una URL binaria directa, lo cual no era cierto.
- `tmpfiles.org` documenta una URL de recurso compartible en su API, no una URL directa garantizada para el flujo `wget -O`.
- `file.io` puede devolver enlaces útiles, pero no se mantuvo en esta ruta estricta porque el objetivo del comando es imprimir únicamente enlaces tratados como descarga directa.

# Decisiones tomadas
- Se retiraron del fallback directo:
  - `temp.sh`
  - `file.io`
  - `tmpfiles.org`
- Se dejó una cadena centrada en enlaces directos o normalizables:
  1. `Litterbox`
  2. `Uguu`
  3. `transfer.sh`
  4. `0x0.st`
- En `transfer.sh` se usa subida `PUT` y se transforma el enlace compartible al alias de descarga directa `/get/...` antes de imprimirlo.
- La salida del CLI ahora dice `Enlace directo:` para dejar clara la garantía buscada.

# Cambios de arquitectura
- `internal/upload` conserva la abstracción `Provider`, pero ahora la lista por defecto se limita a proveedores alineados con el contrato de URL directa del CLI.
- Se añadió `uguuProvider` para parsear la respuesta JSON de Uguu y obtener `files[0].url`.
- Se añadió `transferSHProvider` con:
  - subida `PUT`,
  - encabezado `Max-Days: 1`,
  - conversión de URL de recurso a URL `/get/...`.
- Se agregó `putFile` como helper reutilizable para proveedores no multipart.
- Se agregó `transferDirectURL` para normalizar el enlace de `transfer.sh`.

# Archivos importantes modificados
- `internal/upload/upload.go`
- `internal/upload/upload_test.go`
- `internal/app/app.go`
- `README.md`
- `contexto/03-fix-upload-descarga-directa.md`

# Pruebas añadidas o actualizadas
- Verificación de que los proveedores por defecto son exactamente:
  - `Litterbox`
  - `Uguu`
  - `transfer.sh`
  - `0x0.st`
- Parser de respuesta JSON de Uguu.
- Subida `PUT` de `transfer.sh` y conversión a `/get/...`.
- Preservación de enlaces de `transfer.sh` que ya vienen en formato directo.
- Se mantuvieron las pruebas de fallback y validación temprana de ZIP inexistente.

# Verificación local
- `go test ./...`
- `go build -o gitzip ./cmd/gitzip`
- `GOOS=windows GOARCH=amd64 go build -o gitzip.exe ./cmd/gitzip`

# Limitaciones y notas
- La disponibilidad y políticas de los hosts temporales pueden cambiar; por eso el comando mantiene fallback.
- La validación local confirma el formato y el flujo HTTP con servidores mock. La disponibilidad real de cada proveedor depende de su servicio público en el momento de ejecución.
- El ZIP sigue usando cifrado estándar compatible con `unzip -P`, no cifrado AES moderno.

# Próximos pasos
- Probar `gitzip upload` desde una red real con proyectos de varios tamaños y capturar qué proveedor responde mejor.
- Considerar una opción futura `--provider` para forzar un host concreto al depurar.
- Considerar una validación opcional posterior a la subida, por ejemplo un `HEAD` o comprobación de `Content-Type`, si los proveedores lo soportan de forma estable.
