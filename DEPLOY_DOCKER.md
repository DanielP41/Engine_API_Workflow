# Guía de Despliegue con Docker

Esta guía explica cómo desplegar la aplicación Engine API Workflow utilizando Docker.

## Prerrequisitos

*   [Docker](https://docs.docker.com/get-docker/) instalado y ejecutándose.
*   [Docker Compose](https://docs.docker.com/compose/install/) instalado.
*   (Opcional) `make` instalado para facilitar la ejecución de comandos.

## Despliegue en Producción

Para desplegar la aplicación en un entorno de producción, utilizaremos el archivo de configuración `docker-compose.prod.yml`. Este archivo está optimizado para seguridad y estabilidad (los puertos de base de datos no están expuestos públicamente).

### 1. Configuración de Variables de Entorno

El archivo `docker-compose.prod.yml` utiliza variables de entorno para configuración sensible. Puedes crear un archivo `.env` o exportar las variables antes de ejecutar el comando.

Variables importantes:
*   `MONGO_ROOT_USER` / `MONGO_ROOT_PASSWORD`: Credenciales de la base de datos.
*   `WT_SECRET`: Llave secreta para JWT (¡debe ser larga y segura!).
*   `CORS_ALLOWED_ORIGINS`: Dominios permitidos para acceder a la API.

### 2. Iniciar Servicios

**Usando Make (Recomendado):**
```bash
make docker-prod
```

**Usando Docker Compose directamente:**
```bash
docker-compose -f docker-compose.prod.yml up -d
```

Este comando descargará las imágenes necesarias, construirá la API y levantará todos los servicios en segundo plano (`-d`).

### 3. Verificar el Estado

Puedes verificar que los contenedores estén corriendo con:
```bash
docker ps
```
Deberías ver 3 contenedores: `workflow_api_prod`, `workflow_mongodb_prod`, y `workflow_redis_prod`.

Para ver los logs de la API:
```bash
docker logs -f workflow_api_prod
```

### 4. Detener Servicios

Para detener y remover los contenedores:
```bash
docker-compose -f docker-compose.prod.yml down
```

## Desarrollo Local

Para desarrollo, utiliza el archivo `docker-compose.yml` estándar que expone los puertos de base de datos para fácil acceso.

**Iniciar entorno de desarrollo:**
```bash
make docker-up
# O
docker-compose up -d
```

Esto expondrá:
*   API: `http://localhost:8081`
*   MongoDB: `localhost:27017`
*   Redis: `localhost:6379`

## Solución de Problemas

**Error: puertos en uso**
Si obtienes un error de que el puerto 8081, 27017 o 6379 está en uso, asegúrate de no tener otras instancias corriendo o cambia los puertos en el `docker-compose.yml`.

**Salud de los servicios**
La API tiene un endpoint de salud que Docker usa para verificar el estado:
```bash
curl http://localhost:8081/api/v1/health
```
