# 🚀 Quick Deployment Guide

## Opción 1: Deployment Automático (Recomendado)

### Windows (PowerShell)
```powershell
# 1. Editar configuración de producción
cp .env.example .env.production
# Edita .env.production con tus valores

# 2. Ejecutar script de deployment
.\scripts\deploy-production.ps1
```

### Linux/Mac (Bash)
```bash
# 1. Editar configuración de producción
cp .env.example .env.production
# Edita .env.production con tus valores

# 2. Dar permisos y ejecutar
chmod +x scripts/deploy-production.sh
./scripts/deploy-production.sh
```

## Opción 2: Deployment Manual

### 1. Configurar Variables de Entorno

Crea `.env.production` y configura:

```bash
# CRÍTICO: Cambiar estos valores
JWT_SECRET=tu-secret-super-largo-min-64-caracteres
MONGO_ROOT_PASSWORD=tu-password-seguro-min-32-chars
REDIS_PASSWORD=tu-redis-password-min-32-chars
CORS_ALLOWED_ORIGINS=https://tudominio.com
```

### 2. Generar Secrets Seguros

```bash
# JWT Secret
openssl rand -base64 64

# MongoDB Password
openssl rand -base64 32

# Redis Password
openssl rand -base64 32
```

### 3. Levantar Servicios

```bash
# Cargar variables
export $(cat .env.production | xargs)

# Iniciar servicios
docker-compose -f docker-compose.prod.yml up -d --build

# Ver logs
docker-compose -f docker-compose.prod.yml logs -f
```

### 4. Verificar Deployment

```bash
# Health check
curl http://localhost:8081/api/v1/health

# Crear usuario admin
curl -X POST http://localhost:8081/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Admin",
    "email": "admin@tudominio.com",
    "password": "TuPasswordSeguro123!",
    "role": "admin"
  }'
```

## Verificar JWT Blacklist

```bash
# 1. Login
curl -X POST http://localhost:8081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@tudominio.com","password":"TuPasswordSeguro123!"}'

# 2. Logout (revoca token)
curl -X POST http://localhost:8081/api/v1/auth/logout \
  -H "Authorization: Bearer {tu_token}"

# 3. Verificar en Redis
docker exec -it workflow_redis_prod redis-cli -a ${REDIS_PASSWORD}
> KEYS blacklist:token:*
```

## Comandos Útiles

```bash
# Ver contenedores
docker ps

# Ver logs
docker logs -f workflow_api_prod

# Reiniciar servicios
docker-compose -f docker-compose.prod.yml restart

# Detener servicios
docker-compose -f docker-compose.prod.yml down

# Backup manual
docker exec workflow_mongodb_prod mongodump --out=/backup
```

## 📚 Documentación Completa

Ver `deployment-guide.md` para la guía completa de deployment con:
- Configuración de Nginx
- SSL/TLS
- Monitoreo
- Backups
- Troubleshooting

## ✅ Checklist de Deployment

- [ ] `.env.production` configurado con secrets seguros
- [ ] Servicios levantados con Docker
- [ ] Health check pasando
- [ ] Usuario admin creado
- [ ] JWT Blacklist funcionando
- [ ] Backups configurados
- [ ] (Opcional) Nginx configurado
- [ ] (Opcional) SSL configurado

## 🆘 Problemas Comunes

**API no inicia:**
```bash
docker logs workflow_api_prod
docker restart workflow_api_prod
```

**MongoDB no conecta:**
```bash
docker logs workflow_mongodb_prod
docker exec -it workflow_mongodb_prod mongosh
```

**Redis no conecta:**
```bash
docker exec -it workflow_redis_prod redis-cli -a ${REDIS_PASSWORD} ping
```
