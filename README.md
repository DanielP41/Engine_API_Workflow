# Engine API Workflow

 **API backend empresarial para automatización de flujos de trabajo**

Una solución robusta y escalable que permite a empresas configurar y automatizar procesos internos, desde aprobaciones hasta notificaciones automáticas.

## Características Principales

- **Flujos Configurables**: Crea workflows personalizados por empresa
- **Procesamiento Asíncrono**: Manejo eficiente con Redis y goroutines
- **Integraciones**: Slack, webhooks, APIs externas
- **Autenticación JWT**: Seguridad empresarial con RBAC
- **Monitoreo**: Logs detallados y métricas de rendimiento

## Stack Tecnológico

- **Backend**: Go (Fiber framework)
- **Base de Datos**: MongoDB
- **Cola de Mensajes**: Redis
- **Contenerización**: Docker
- **CI/CD**: GitHub Actions

## Estado del Proyecto

🔄 **En desarrollo activo** - Siguiendo metodología por fases

- [x] Planificación y arquitectura
- [x] Setup inicial y configuración
- [x] Core API y autenticación
- [x] Sistema de workflows
- [ ] 🔄Procesamiento asíncrono
- [ ] Integraciones externas
- [ ] Testing y documentación
- [ ] Deployment y monitoreo

## Funcionalidades Implementadas
**Sistema de Autenticación**

 Registro y login de usuarios
 JWT con access y refresh tokens
 Blacklist automática de tokens
 Rate limiting por usuario
 Roles y permisos (RBAC)

**Gestión de Workflows**

 CRUD completo de workflows
 Sistema de pasos condicionales
 Triggers múltiples (manual, webhook, scheduled)
 Versionado de workflows
 Clonación y templates

**Monitoreo y Logs**

 Auditoría completa de acciones
 Logs estructurados
 Métricas de ejecución
 Health checks automatizados
 Estadísticas en tiempo real

**Procesamiento Asíncrono**

 Sistema de colas con prioridades
 Jobs diferidos y programados
 Manejo básico de reintentos
 Workers distribuidos (en desarrollo)

**Arquitectura**
El proyecto implementa Clean Architecture con separación de responsabilidades:

Handlers: Capa de presentación (REST API)
Services: Lógica de negocio
Repositories: Acceso a datos
Models: Entidades de dominio


## Quick Start

```bash
# Clonar repositorio
git clone https://github.com/DanielP41/Engine_API_Workflow.git
cd Engine_API_Workflow

# Configurar entorno
cp .env.example .env

# Levantar servicios con Docker
docker-compose up -d

# Ejecutar API
make run
