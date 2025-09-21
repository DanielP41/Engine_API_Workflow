#!/bin/bash

# ================================
# Script de Administración para el Sistema de Notificaciones
# Versión: 2.0.0
# Uso: ./notification_admin.sh [comando] [opciones]
# ================================

set -e

# ================================
# CONFIGURACIÓN POR DEFECTO
# ================================

API_BASE_URL="${API_BASE_URL:-http://localhost:8081}"
API_TOKEN="${API_TOKEN:-}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
CONFIG_FILE="${CONFIG_FILE:-./notification_admin.conf}"
LOG_FILE="${LOG_FILE:-./notification_admin.log}"
DEBUG="${DEBUG:-false}"

# ================================
# COLORES Y EMOJIS
# ================================

# Colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m' # No Color

# Emojis
SUCCESS="✅"
ERROR="❌"
WARNING="⚠️"
INFO="ℹ️"
ROCKET="🚀"
EMAIL="📧"
GEAR="⚙️"
CHART="📊"
CLOCK="⏰"
FIRE="🔥"
CLEAN="🧹"
LOCK="🔒"
KEY="🔑"

# ================================
# FUNCIONES DE UTILIDAD
# ================================

# Función para logging
log() {
    local level="$1"
    local message="$2"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] [$level] $message" >> "$LOG_FILE"
    
    if [ "$DEBUG" = "true" ]; then
        echo -e "${CYAN}[DEBUG]${NC} $message" >&2
    fi
}

# Función para mostrar mensajes con colores
print_success() {
    echo -e "${GREEN}${SUCCESS} $1${NC}"
    log "SUCCESS" "$1"
}

print_error() {
    echo -e "${RED}${ERROR} $1${NC}" >&2
    log "ERROR" "$1"
}

print_warning() {
    echo -e "${YELLOW}${WARNING} $1${NC}"
    log "WARNING" "$1"
}

print_info() {
    echo -e "${BLUE}${INFO} $1${NC}"
    log "INFO" "$1"
}

print_header() {
    echo -e "${WHITE}================================${NC}"
    echo -e "${WHITE}$1${NC}"
    echo -e "${WHITE}================================${NC}"
}

# Función para mostrar spinner
show_spinner() {
    local pid=$1
    local delay=0.1
    local spinstr='|/-\'
    while [ "$(ps a | awk '{print $1}' | grep $pid)" ]; do
        local temp=${spinstr#?}
        printf " [%c]  " "$spinstr"
        local spinstr=$temp${spinstr%"$temp"}
        sleep $delay
        printf "\b\b\b\b\b\b"
    done
    printf "    \b\b\b\b"
}

# ================================
# FUNCIONES DE CONFIGURACIÓN
# ================================

# Cargar configuración desde archivo
load_config() {
    if [ -f "$CONFIG_FILE" ]; then
        source "$CONFIG_FILE"
        print_info "Configuración cargada desde $CONFIG_FILE"
    fi
}

# Guardar configuración
save_config() {
    cat > "$CONFIG_FILE" << EOF
# Configuración del Administrador de Notificaciones
API_BASE_URL="$API_BASE_URL"
API_TOKEN="$API_TOKEN"
ADMIN_EMAIL="$ADMIN_EMAIL"
DEBUG="$DEBUG"

# Generado el $(date)
EOF
    print_success "Configuración guardada en $CONFIG_FILE"
}

# Configuración interactiva
setup_config() {
    print_header "Configuración Inicial"
    
    echo -n "URL base de la API [$API_BASE_URL]: "
    read input
    [ -n "$input" ] && API_BASE_URL="$input"
    
    echo -n "Token de autenticación: "
    read -s input
    echo
    [ -n "$input" ] && API_TOKEN="$input"
    
    echo -n "Email del administrador [$ADMIN_EMAIL]: "
    read input
    [ -n "$input" ] && ADMIN_EMAIL="$input"
    
    echo -n "Habilitar modo debug? [y/N]: "
    read input
    if [[ $input =~ ^[Yy]$ ]]; then
        DEBUG="true"
    else
        DEBUG="false"
    fi
    
    save_config
    print_success "Configuración completada"
}

# ================================
# FUNCIONES DE VALIDACIÓN
# ================================

# Verificar dependencias
check_dependencies() {
    local missing=()
    
    command -v curl >/dev/null 2>&1 || missing+=("curl")
    command -v jq >/dev/null 2>&1 || missing+=("jq")
    command -v date >/dev/null 2>&1 || missing+=("date")
    
    if [ ${#missing[@]} -ne 0 ]; then
        print_error "Dependencias faltantes: ${missing[*]}"
        echo "Instala las dependencias faltantes:"
        echo "  Ubuntu/Debian: sudo apt-get install ${missing[*]}"
        echo "  CentOS/RHEL: sudo yum install ${missing[*]}"
        echo "  macOS: brew install ${missing[*]}"
        exit 1
    fi
}

# Validar configuración
validate_config() {
    if [ -z "$API_BASE_URL" ]; then
        print_error "API_BASE_URL no está configurado"
        echo "Ejecuta: $0 setup"
        exit 1
    fi
    
    if [ -z "$API_TOKEN" ]; then
        print_error "API_TOKEN no está configurado"
        echo "Ejecuta: $0 setup"
        exit 1
    fi
}

# ================================
# FUNCIONES DE API
# ================================

# Función principal para hacer requests a la API
api_request() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    local timeout="${4:-30}"
    
    log "API_REQUEST" "$method $endpoint"
    
    local curl_opts=(
        -s
        -w "%{http_code}|||%{time_total}|||%{size_download}"
        -H "Authorization: Bearer $API_TOKEN"
        -H "Content-Type: application/json"
        -H "User-Agent: NotificationAdmin/2.0.0"
        --connect-timeout 10
        --max-time "$timeout"
    )
    
    case "$method" in
        "POST"|"PUT")
            curl_opts+=(-X "$method")
            if [ -n "$data" ]; then
                curl_opts+=(-d "$data")
            fi
            ;;
        "DELETE")
            curl_opts+=(-X "DELETE")
            ;;
        "GET")
            # GET es el método por defecto
            ;;
        *)
            print_error "Método HTTP no soportado: $method"
            return 1
            ;;
    esac
    
    local response=$(curl "${curl_opts[@]}" "$API_BASE_URL$endpoint" 2>/dev/null || echo "|||CURL_ERROR|||")
    
    if [[ "$response" == *"CURL_ERROR"* ]]; then
        print_error "Error de conexión con la API"
        return 1
    fi
    
    local http_code=$(echo "$response" | cut -d'|||' -f1)
    local time_total=$(echo "$response" | cut -d'|||' -f2)
    local size_download=$(echo "$response" | cut -d'|||' -f3)
    local body=$(echo "$response" | cut -d'|||' -f4-)
    
    log "API_RESPONSE" "HTTP $http_code - ${time_total}s - ${size_download} bytes"
    
    if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
        echo "$body"
        return 0
    else
        print_error "Error HTTP $http_code"
        if command -v jq >/dev/null 2>&1 && echo "$body" | jq empty 2>/dev/null; then
            echo "$body" | jq -r '.message // .error // "Error desconocido"' >&2
        else
            echo "$body" >&2
        fi
        return 1
    fi
}

# ================================
# COMANDOS PRINCIPALES
# ================================

# Verificar estado del sistema
cmd_status() {
    print_header "Estado del Sistema de Notificaciones"
    
    print_info "Verificando conexión con la API..."
    
    if response=$(api_request "GET" "/api/v1/health/notifications"); then
        echo "$response" | jq -r '
            "'" ${SUCCESS}"' Estado del servicio: \(.service // \"unknown\")",
            "'" ${EMAIL}"' Servicio de email: \(.email_service // \"unknown\")",
            "'" ${GEAR}"' Base de datos: \(.database // \"unknown\")",
            "'" ${CHART}"' Templates en cache: \(.template_cache // 0)",
            "'" ${CLOCK}"' Último procesamiento: \(.last_processing // \"nunca\")"
        ' 2>/dev/null || {
            print_success "Sistema disponible"
            echo "$response"
        }
    else
        print_error "Sistema no disponible"
        return 1
    fi
    
    echo
    print_info "Obteniendo estadísticas rápidas..."
    
    if response=$(api_request "GET" "/api/v1/notifications/stats/service"); then
        echo "$response" | jq -r '
            .data |
            "'" ${CHART}"' Emails enviados hoy: \(.emails_sent_today // 0)",
            "'" ${ERROR}"' Emails fallidos hoy: \(.emails_failed_today // 0)",
            "'" ${CLOCK}"' Tiempo de actividad: \(.uptime // \"desconocido\")",
            "'" ${GEAR}"' Templates activos: \(.active_templates // 0)"
        ' 2>/dev/null || echo "Estadísticas no disponibles"
    fi
}

# Mostrar estadísticas detalladas
cmd_stats() {
    local days="${1:-7}"
    local time_range="${days}d"
    
    print_header "Estadísticas de Notificaciones ($days días)"
    
    if response=$(api_request "GET" "/api/v1/notifications/stats?time_range=$time_range"); then
        echo "$response" | jq -r '
            .data |
            "'" ${CHART}"' ESTADÍSTICAS GENERALES:",
            "   Total notificaciones: \(.total_notifications // 0)",
            "   Tasa de éxito: \((.success_rate * 100 | floor) // 0)%",
            "   Promedio de reintentos: \(.average_retries // 0)",
            "",
            "'" ${GEAR}"' POR ESTADO:",
            (.by_status | to_entries | .[] | "   \(.key): \(.value)"),
            "",
            "'" ${EMAIL}"' POR TIPO:",
            (.by_type | to_entries | .[] | "   \(.key): \(.value)"),
            "",
            "'" ${FIRE}"' POR PRIORIDAD:",
            (.by_priority | to_entries | .[] | "   \(.key): \(.value)")
        '
    else
        print_error "No se pudieron obtener las estadísticas"
        return 1
    fi
}

# Enviar email de prueba
cmd_send_test() {
    local email="${1:-$ADMIN_EMAIL}"
    
    print_info "Enviando email de prueba a $email..."
    
    local data=$(cat << EOF
{
    "type": "custom",
    "priority": "normal",
    "to": ["$email"],
    "subject": "🧪 Email de Prueba - Sistema de Notificaciones",
    "body": "Este es un email de prueba del sistema de notificaciones.\n\nFecha: $(date)\nServidor: $(hostname)\nScript: notification_admin.sh v2.0.0\n\nSi recibes este email, el sistema está funcionando correctamente.\n\n${SUCCESS} ¡Prueba exitosa!",
    "is_html": false
}
EOF
)
    
    if response=$(api_request "POST" "/api/v1/notifications/send" "$data"); then
        local notification_id=$(echo "$response" | jq -r '.data.id // .data.notification_id // "unknown"')
        print_success "Email de prueba enviado"
        echo "   ${INFO} ID de notificación: $notification_id"
        echo "   ${EMAIL} Destinatario: $email"
    else
        print_error "No se pudo enviar el email de prueba"
        return 1
    fi
}

# Procesar notificaciones pendientes
cmd_process_pending() {
    print_info "Procesando notificaciones pendientes..."
    
    if api_request "POST" "/api/v1/notifications/admin/process-pending" "" 60 >/dev/null; then
        print_success "Notificaciones pendientes procesadas"
    else
        print_error "Error al procesar notificaciones pendientes"
        return 1
    fi
}

# Reintentar notificaciones fallidas
cmd_retry_failed() {
    print_info "Reintentando notificaciones fallidas..."
    
    if api_request "POST" "/api/v1/notifications/admin/retry-failed" "" 60 >/dev/null; then
        print_success "Reintentos de notificaciones fallidas iniciados"
    else
        print_error "Error al reintentar notificaciones fallidas"
        return 1
    fi
}

# Limpiar notificaciones antiguas
cmd_cleanup() {
    local days="${1:-90}"
    
    print_warning "Esto eliminará notificaciones de más de $days días"
    echo -n "¿Continuar? [y/N]: "
    read confirm
    
    if [[ ! $confirm =~ ^[Yy]$ ]]; then
        print_info "Operación cancelada"
        return 0
    fi
    
    print_info "Limpiando notificaciones de más de $days días..."
    
    local hours=$((days * 24))
    if response=$(api_request "DELETE" "/api/v1/notifications/admin/cleanup?older_than=${hours}h" "" 120); then
        local deleted_count=$(echo "$response" | jq -r '.data.deleted_count // 0')
        print_success "Limpieza completada"
        echo "   ${CLEAN} Notificaciones eliminadas: $deleted_count"
    else
        print_error "Error durante la limpieza"
        return 1
    fi
}

# Listar notificaciones recientes
cmd_list_notifications() {
    local limit="${1:-20}"
    
    print_header "Notificaciones Recientes (últimas $limit)"
    
    if response=$(api_request "GET" "/api/v1/notifications?limit=$limit&sort=created_at&order=desc"); then
        echo "$response" | jq -r '
            .data.notifications[] |
            "'" ${EMAIL}"' \(.id[0:8]...) | \(.status) | \(.type) | \(.priority)",
            "   Para: \(.to | join(\", \"))",
            "   Asunto: \(.subject[0:60])\(if (.subject | length) > 60 then \"...\" else \"\" end)",
            "   Creado: \(.created_at)",
            (if .sent_at then "   '" ${SUCCESS}"' Enviado: \(.sent_at)" else "" end),
            (if .errors and (.errors | length) > 0 then "   '" ${ERROR}"' Errores: \(.errors | length)" else "" end),
            ""
        '
    else
        print_error "No se pudieron obtener las notificaciones"
        return 1
    fi
}

# Listar templates disponibles
cmd_list_templates() {
    print_header "Templates Disponibles"
    
    if response=$(api_request "GET" "/api/v1/notifications/templates"); then
        echo "$response" | jq -r '
            .data.templates[] |
            "'" ${GEAR}"' \(.name) (v\(.version)) | \(.type) | \(.language)",
            "   Descripción: \(.description // \"Sin descripción\")",
            "   Tags: \(.tags | join(\", \"))",
            "   Creado: \(.created_at) por \(.created_by)",
            "   Estado: \(if .is_active then \"'" ${SUCCESS}"' Activo\" else \"'" ${ERROR}"' Inactivo\" end)",
            ""
        '
    else
        print_error "No se pudieron obtener los templates"
        return 1
    fi
}

# Crear template desde archivo JSON
cmd_create_template() {
    local file="$1"
    
    if [ -z "$file" ]; then
        print_error "Especifica el archivo JSON del template"
        echo "Uso: $0 create-template template.json"
        return 1
    fi
    
    if [ ! -f "$file" ]; then
        print_error "Archivo no encontrado: $file"
        return 1
    fi
    
    # Validar JSON
    if ! jq empty "$file" 2>/dev/null; then
        print_error "El archivo no contiene JSON válido"
        return 1
    fi
    
    print_info "Creando template desde $file..."
    
    local data=$(cat "$file")
    if response=$(api_request "POST" "/api/v1/notifications/templates" "$data"); then
        local template_id=$(echo "$response" | jq -r '.data.id // "unknown"')
        local template_name=$(echo "$response" | jq -r '.data.name // "unknown"')
        print_success "Template '$template_name' creado"
        echo "   ${INFO} ID: $template_id"
    else
        print_error "No se pudo crear el template"
        return 1
    fi
}

# Probar configuración SMTP
cmd_test_config() {
    print_info "Probando configuración SMTP..."
    
    if api_request "POST" "/api/v1/notifications/admin/test-config" "" 30 >/dev/null; then
        print_success "Configuración SMTP funciona correctamente"
    else
        print_error "Error en la configuración SMTP"
        return 1
    fi
}

# Crear templates por defecto
cmd_create_defaults() {
    print_info "Creando templates por defecto del sistema..."
    
    if api_request "POST" "/api/v1/notifications/admin/default-templates" "" 60 >/dev/null; then
        print_success "Templates por defecto creados"
    else
        print_error "Error al crear templates por defecto"
        return 1
    fi
}

# Monitoreo en tiempo real
cmd_monitor() {
    print_header "Monitoreo en Tiempo Real"
    print_info "Presiona Ctrl+C para detener"
    echo
    
    trap 'echo; print_info "Monitoreo detenido"; exit 0' INT
    
    while true; do
        clear
        echo -e "${WHITE}=== MONITOREO DEL SISTEMA DE NOTIFICACIONES ===${NC}"
        echo "Actualización: $(date)"
        echo
        
        # Estado del sistema
        if response=$(api_request "GET" "/api/v1/health/notifications" "" 5); then
            echo "$response" | jq -r '
                "'" ${GEAR}"' Estado: \(.service // \"unknown\")",
                "'" ${EMAIL}"' Email: \(.email_service // \"unknown\")",
                "'" ${CHART}"' Templates: \(.template_cache // 0)"
            ' 2>/dev/null || echo "${INFO} Sistema activo"
        else
            echo "${ERROR} Sistema no disponible"
        fi
        
        echo
        
        # Estadísticas rápidas
        if response=$(api_request "GET" "/api/v1/notifications/stats/service" "" 5); then
            echo "$response" | jq -r '
                .data |
                "'" ${CHART}"' Enviados hoy: \(.emails_sent_today // 0)",
                "'" ${ERROR}"' Fallidos hoy: \(.emails_failed_today // 0)"
            ' 2>/dev/null
        fi
        
        echo
        echo "Próxima actualización en 10 segundos..."
        sleep 10
    done
}

# Generar reporte
cmd_report() {
    local days="${1:-30}"
    local output_file="notification_report_$(date +%Y%m%d_%H%M%S).txt"
    
    print_info "Generando reporte de $days días..."
    
    exec 3>"$output_file"
    
    {
        echo "REPORTE DEL SISTEMA DE NOTIFICACIONES"
        echo "======================================"
        echo "Generado: $(date)"
        echo "Período: Últimos $days días"
        echo "Script: notification_admin.sh v2.0.0"
        echo
        
        echo "ESTADO DEL SISTEMA"
        echo "=================="
        if response=$(api_request "GET" "/api/v1/health/notifications"); then
            echo "$response" | jq -r '
                "Estado del servicio: \(.service // \"unknown\")",
                "Servicio de email: \(.email_service // \"unknown\")",
                "Base de datos: \(.database // \"unknown\")",
                "Templates en cache: \(.template_cache // 0)"
            ' 2>/dev/null || echo "Estado: Sistema activo"
        else
            echo "Estado: Sistema no disponible"
        fi
        
        echo
        echo "ESTADÍSTICAS DETALLADAS"
        echo "======================="
        if response=$(api_request "GET" "/api/v1/notifications/stats?time_range=${days}d"); then
            echo "$response" | jq -r '
                .data |
                "Total notificaciones: \(.total_notifications // 0)",
                "Tasa de éxito: \((.success_rate * 100 | floor) // 0)%",
                "Promedio de reintentos: \(.average_retries // 0)",
                "",
                "POR ESTADO:",
                (.by_status | to_entries | .[] | "  \(.key): \(.value)"),
                "",
                "POR TIPO:",
                (.by_type | to_entries | .[] | "  \(.key): \(.value)"),
                "",
                "POR PRIORIDAD:",
                (.by_priority | to_entries | .[] | "  \(.key): \(.value)")
            '
        else
            echo "No se pudieron obtener estadísticas"
        fi
        
        echo
        echo "TEMPLATES DISPONIBLES"
        echo "===================="
        if response=$(api_request "GET" "/api/v1/notifications/templates"); then
            echo "$response" | jq -r '
                .data.templates[] |
                "\(.name) (v\(.version)) - \(.type) - \(if .is_active then \"Activo\" else \"Inactivo\" end)"
            '
        else
            echo "No se pudieron obtener templates"
        fi
        
    } >&3
    
    exec 3>&-
    
    print_success "Reporte generado: $output_file"
}

# ================================
# FUNCIONES AVANZADAS
# ================================

# Backup de templates
cmd_backup_templates() {
    local backup_file="templates_backup_$(date +%Y%m%d_%H%M%S).json"
    
    print_info "Creando backup de templates..."
    
    if response=$(api_request "GET" "/api/v1/notifications/templates"); then
        echo "$response" > "$backup_file"
        print_success "Backup creado: $backup_file"
    else
        print_error "Error al crear backup"
        return 1
    fi
}

# Restaurar templates desde backup
cmd_restore_templates() {
    local backup_file="$1"
    
    if [ -z "$backup_file" ]; then
        print_error "Especifica el archivo de backup"
        echo "Uso: $0 restore-templates backup.json"
        return 1
    fi
    
    if [ ! -f "$backup_file" ]; then
        print_error "Archivo de backup no encontrado: $backup_file"
        return 1
    fi
    
    print_warning "Esto puede sobrescribir templates existentes"
    echo -n "¿Continuar? [y/N]: "
    read confirm
    
    if [[ ! $confirm =~ ^[Yy]$ ]]; then
        print_info "Operación cancelada"
        return 0
    fi
    
    print_info "Restaurando templates desde $backup_file..."
    
    # Procesar cada template del backup
    local count=0
    local success=0
    
    while IFS= read -r template; do
        if echo "$template" | jq empty 2>/dev/null; then
            ((count++))
            if api_request "POST" "/api/v1/notifications/templates" "$template" >/dev/null 2>&1; then
                ((success++))
            fi
        fi
    done < <(jq -c '.data.templates[]' "$backup_file" 2>/dev/null)
    
    print_success "Restauración completada"
    echo "   ${INFO} Templates procesados: $count"
    echo "   ${SUCCESS} Templates restaurados: $success"
}

# Modo interactivo
cmd_interactive() {
    print_header "Modo Interactivo del Administrador de Notificaciones"
    
    while true; do
        echo
        echo "Comandos disponibles:"
        echo "  1) Ver estado del sistema"
        echo "  2) Ver estadísticas"
        echo "  3) Enviar email de prueba"
        echo "  4) Procesar notificaciones pendientes"
        echo "  5) Listar notificaciones recientes"
        echo "  6) Listar templates"
        echo "  7) Probar configuración SMTP"
        echo "  8) Generar reporte"
        echo "  9) Configurar sistema"
        echo "  0) Salir"
        echo
        echo -n "Selecciona una opción [0-9]: "
        read choice
        
        case "$choice" in
            1) cmd_status ;;
            2) echo -n "Días para estadísticas [7]: "; read days; cmd_stats "${days:-7}" ;;
            3) echo -n "Email para prueba [$ADMIN_EMAIL]: "; read email; cmd_send_test "${email:-$ADMIN_EMAIL}" ;;
            4) cmd_process_pending ;;
            5) cmd_list_notifications ;;
            6) cmd_list_templates ;;
            7) cmd_test_config ;;
            8) echo -n "Días para reporte [30]: "; read days; cmd_report "${days:-30}" ;;
            9) setup_config ;;
            0) print_info "¡Hasta luego!"; break ;;
            *) print_warning "Opción no válida" ;;
        esac
    done
}

# ================================
# FUNCIÓN DE AYUDA
# ================================

show_help() {
    cat << EOF
${WHITE}Sistema de Administración de Notificaciones v2.0.0${NC}

${YELLOW}COMANDOS PRINCIPALES:${NC}
    ${GREEN}status${NC}                    - Verificar estado del sistema
    ${GREEN}stats [days]${NC}             - Mostrar estadísticas (default: 7 días)
    ${GREEN}send-test [email]${NC}        - Enviar email de prueba
    ${GREEN}process-pending${NC}          - Procesar notificaciones pendientes
    ${GREEN}retry-failed${NC}             - Reintentar notificaciones fallidas
    ${GREEN}cleanup [days]${NC}           - Limpiar notificaciones antiguas (default: 90 días)

${YELLOW}GESTIÓN DE NOTIFICACIONES:${NC}
    ${GREEN}list [limit]${NC}             - Listar notificaciones recientes
    ${GREEN}monitor${NC}                  - Monitoreo en tiempo real

${YELLOW}GESTIÓN DE TEMPLATES:${NC}
    ${GREEN}list-templates${NC}           - Listar templates disponibles
    ${GREEN}create-template <file>${NC}   - Crear template desde archivo JSON
    ${GREEN}create-defaults${NC}          - Crear templates por defecto del sistema
    ${GREEN}backup-templates${NC}         - Crear backup de templates
    ${GREEN}restore-templates <file>${NC} - Restaurar templates desde backup

${YELLOW}CONFIGURACIÓN Y TESTING:${NC}
    ${GREEN}test-config${NC}              - Probar configuración SMTP
    ${GREEN}setup${NC}                    - Configuración interactiva
    ${GREEN}interactive${NC}              - Modo interactivo

${YELLOW}REPORTES Y ANÁLISIS:${NC}
    ${GREEN}report [days]${NC}            - Generar reporte detallado

${YELLOW}EJEMPLOS:${NC}
    $0 status
    $0 stats 30
    $0 send-test usuario@ejemplo.com
    $0 cleanup 60
    $0 create-template welcome_template.json
    $0 monitor

${YELLOW}VARIABLES DE ENTORNO:${NC}
    ${CYAN}API_BASE_URL${NC}    - URL base de la API (default: http://localhost:8081)
    ${CYAN}API_TOKEN${NC}       - Token JWT para autenticación
    ${CYAN}ADMIN_EMAIL${NC}     - Email del administrador (default: admin@example.com)
    ${CYAN}CONFIG_FILE${NC}     - Archivo de configuración (default: ./notification_admin.conf)
    ${CYAN}DEBUG${NC}           - Habilitar modo debug (default: false)

${YELLOW}ARCHIVOS:${NC}
    ${CYAN}$CONFIG_FILE${NC} - Configuración del script
    ${CYAN}$LOG_FILE${NC} - Log de operaciones

Para más información, visita: https://github.com/yourorg/engine-workflow

EOF
}

# ================================
# FUNCIÓN PRINCIPAL
# ================================

main() {
    local command="$1"
    shift 2>/dev/null || true
    
    # Cargar configuración
    load_config
    
    # Comandos que no requieren validación
    case "$command" in
        "setup"|"help"|"-h"|"--help"|"")
            ;;
        *)
            check_dependencies
            validate_config
            ;;
    esac
    
    # Procesar comandos
    case "$command" in
        ""|"help"|"-h"|"--help")
            show_help
            ;;
        "setup")
            setup_config
            ;;
        "status")
            cmd_status
            ;;
        "stats")
            cmd_stats "$@"
            ;;
        "send-test")
            cmd_send_test "$@"
            ;;
        "process-pending")
            cmd_process_pending
            ;;
        "retry-failed")
            cmd_retry_failed
            ;;
        "cleanup")
            cmd_cleanup "$@"
            ;;
        "list"|"list-notifications")
            cmd_list_notifications "$@"
            ;;
        "list-templates"|"templates")
            cmd_list_templates
            ;;
        "create-template")
            cmd_create_template "$@"
            ;;
        "create-defaults")
            cmd_create_defaults
            ;;
        "test-config")
            cmd_test_config
            ;;
        "monitor")
            cmd_monitor
            ;;
        "report")
            cmd_report "$@"
            ;;
        "backup-templates")
            cmd_backup_templates
            ;;
        "restore-templates")
            cmd_restore_templates "$@"
            ;;
        "interactive")
            cmd_interactive
            ;;
        *)
            print_error "Comando no reconocido: $command"
            echo "Ejecuta '$0 help' para ver todos los comandos disponibles"
            exit 1
            ;;
    esac
}

# ================================
# INICIALIZACIÓN
# ================================

# Crear directorio de logs si no existe
mkdir -p "$(dirname "$LOG_FILE")"

# Ejecutar función principal
main "$@"