# Copia vendorizada mínima de `github.com/yeka/zip`

Esta carpeta conserva el soporte necesario para escribir y leer ZIP con
**Standard Zip Encryption** (ZipCrypto), suficiente para mantener compatibilidad
con `unzip -P` en el comando `gitzip upload`.

Se tomó como base el commit `03d6312748a9d6e0bc0c9a7275385c09f06d9c14` de
`github.com/yeka/zip` y se redujo la capa de cifrado a ZipCrypto para evitar
arrastrar dependencias AES que este proyecto no usa.

La licencia original se conserva en `LICENSE`.
