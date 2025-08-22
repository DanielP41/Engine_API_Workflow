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

- [x] ~~Planificación y arquitectura~~
- [ ] 🔄 Setup inicial y configuración
- [ ] Core API y autenticación
- [ ] Sistema de workflows
- [ ] Procesamiento asíncrono
- [ ] Integraciones externas
- [ ] Testing y documentación
- [ ] Deployment y monitoreo

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