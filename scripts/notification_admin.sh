# Script de administración para el sistema de notificaciones
# Uso: ./notification_admin.sh [comando] [opciones]

set -e

# Configuración por defecto
API_BASE_URL="${API_BASE_URL:-http://localhost:8081}"
API_TOKEN="${API_TOKEN:-}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Función para mostrar ayuda
show_help() {
    cat << EOF
Sistema de Administración de Notificaciones

COMANDOS DISPONIBLES:
    status              - Verificar estado del sistema
    stats [days]        - Mostrar estadísticas (default: 7 días)
    send-test          - Enviar email de prueba
    process-pending    - Procesar notificaciones pendientes
    retry-failed       - Reintentar notificaciones fallidas
    cleanup [days]     - Limpiar notificaciones antiguas (default: 90 días)
    list-notifications - Listar notificaciones recientes
    list-templates     - Listar templates disponibles
    create-template    - Crear template desde archivo JSON
    test-config        - Probar configuración SMTP
    monitor            - Monitoreo en tiempo real

EJEMPLOS:
    $0 status
    $0 stats 30
    $0 send-test usuario@ejemplo.com
    $0 cleanup 60
    $0 create-template template.json

VARIABLES DE ENTORNO:
    API_BASE_URL    - URL base de la API (default: http://localhost:8081)
    API_TOKEN       - Token JWT para autenticación
    ADMIN_EMAIL     - Email del administrador (default: admin@example.com)

EOF
}

# Función para verificar dependencias
check_dependencies() {
    if ! command -v curl &> /dev/null; then
        echo -e "${RED}Error: curl no está instalado${NC}"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        echo -e "${RED}Error: jq no está instalado${NC}"
        exit 1
    fi
    
    if [ -z "$API_TOKEN" ]; then
        echo -e "${RED}Error: API_TOKEN no está configurado${NC}"
        echo "Configura la variable de entorno API_TOKEN con tu JWT token"
        exit 1
    fi
}

# Función para hacer requests a la API
api_request() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    
    local curl_opts=(-s -w "%{http_code}" -H "Authorization: Bearer $API_TOKEN" -H "Content-Type: application/json")
    
    if [ "$method" = "POST" ] || [ "$method" = "PUT" ]; then
        curl_opts+=(-X "$method" -d "$data")
    elif [ "$method" = "DELETE" ]; then
        curl_opts+=(-X "DELETE")
    fi
    
    local response=$(curl "${curl_opts[@]}" "$API_BASE_URL$endpoint")
    local http_code="${response: -3}"
    local body="${response%???}"
    
    if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
        echo "$body"
        return 0
    else
        echo -e "${RED}Error: HTTP $http_code${NC}" >&2
        echo "$body" | jq -r '.message // .error // "Error desconocido"' >&2
        return 1
    fi
}

# Verificar estado del sistema
check_status() {
    echo -e "${BLUE}Verificando estado del sistema...${NC}"
    
    if response=$(api_request "GET" "/health/notifications"); then
        echo "$response" | jq -r '
            if .status == "healthy" then
                "✅ Sistema: \(.status)"
            else
                "❌ Sistema: \(.status)"
            end,
            "📧 Email: \(.email)",
            "⚙️  Worker: \(.worker)",
            "📊 Colas:",
            "   • Pendientes: \(.queue_stats.pending // 0)",
            "   • Procesando: \(.queue_stats.processing // 0)",
            "   • Fallidas: \(.queue_stats.failed // 0)",
            "   • Reintentos: \(.queue_stats.retry // 0)",
            "   • Programadas: \(.queue_stats.scheduled // 0)"
        '
    else
        echo -e "${RED}❌ Sistema no disponible${NC}"
        return 1
    fi
}

# Mostrar estadísticas
show_stats() {
    local days="${1:-7}"
    echo -e "${BLUE}Estadísticas de los últimos $days días...${NC}"
    
    local time_range="${days}d"
    if response=$(api_request "GET" "/api/v1/notifications/stats?time_range=$time_range"); then
        echo "$response" | jq -r '
            .data | 
            "📊 ESTADÍSTICAS GENERALES:",
            "   Total notificaciones: \(.total_notifications)",
            "   Tasa de éxito: \((.success_rate * 100 | floor))%",
            "   Promedio reintentos: \(.average_retries)",
            "",
            "📈 POR ESTADO:",
            (.by_status | to_entries | .[] | "   \(.key): \(.value)"),
            "",
            "📂 POR TIPO:",
            (.by_type | to_entries | .[] | "   \(.key): \(.value)"),
            "",
            "⚡ POR PRIORIDAD:",
            (.by_priority | to_entries | .[] | "   \(.key): \(.value)")
        '
    fi
}

# Enviar email de prueba
send_test_email() {
    local email="${1:-$ADMIN_EMAIL}"
    echo -e "${BLUE}Enviando email de prueba a $email...${NC}"
    
    local data=$(cat << EOF
{
    "type": "custom",
    "priority": "normal",
    "to": ["$email"],
    "subject": "🧪 Email de Prueba - Sistema de Notificaciones",
    "body": "Este es un email de prueba del sistema de notificaciones.\n\nFecha: $(date)\nServidor: $(hostname)\n\nSi recibes este email, el sistema está funcionando correctamente.",
    "is_html": false
}
EOF
)
    
    if response=$(api_request "POST" "/api/v1/notifications/send" "$data"); then
        local notification_id=$(echo "$response" | jq -r '.data.notification_id')
        echo -e "${GREEN}✅ Email de prueba enviado${NC}"
        echo "ID de notificación: $notification_id"
    fi
}

# Procesar notificaciones pendientes
process_pending() {
    echo -e "${BLUE}Procesando notificaciones pendientes...${NC}"
    
    if api_request "POST" "/api/v1/notifications/admin/process-pending" "" > /dev/null; then
        echo -e "${GREEN}✅ Notificaciones pendientes procesadas${NC}"
    fi
}

# Reintentar notificaciones fallidas
retry_failed() {
    echo -e "${BLUE}Reintentando notificaciones fallidas...${NC}"
    
    if api_request "POST" "/api/v1/notifications/admin/retry-failed" "" > /dev/null; then
        echo -e "${GREEN}✅ Reintentos iniciados${NC}"
    fi
}

# Limpiar notificaciones antiguas
cleanup_old() {
    local days="${1:-90}"
    echo -e "${BLUE}Limpiando notificaciones de más de $days días...${NC}"
    
    local hours=$((days * 24))
    if response=$(api_request "DELETE" "/api/v1/notifications/admin/cleanup?older_than=${hours}h"); then
        local deleted_count=$(echo "$response" | jq -r '.data.deleted_count')
        echo -e "${GREEN}✅ Limpieza completada: $deleted_count notificaciones eliminadas${NC}"
    fi
}

# Listar notificaciones recientes
list_notifications() {
    echo -e "${BLUE}Notificaciones recientes...${NC}"
    
    if response=$(api_request "GET" "/api/v1/notifications?limit=10"); then
        echo "$response" | jq -r '
            .data.notifications[] | 
            "🔔 \(.id[0:8]...) | \(.status) | \(.type) | \(.priority)",
            "   📧 Para: \(.to | join(", "))",
            "   📝 Asunto: \(.subject)",
            "   📅 Creado: \(.created_at)",
            if .sent_at then "   ✅ Enviado: \(.sent_at)" else "" end,
            if .errors and (.errors | length) > 0 then "   ❌ Errores: \(.errors | length)" else "" end,
            ""
        '
    fi
}

# Listar templates
list_templates() {
    echo -e "${BLUE}Templates disponibles...${NC}"
    
    if response=$(api_request "GET" "/api/v1/notifications/templates"); then
        echo "$response" | jq -r '
            .data.templates[] | 
            "📄 \(.name) (v\(.version)) | \(.type) | \(.language)",
            "   📝 \(.description // "Sin descripción")",
            "   🏷️  \(.tags | join(", "))",
            "   📅 Creado: \(.created_at) por \(.created_by)",
            "   \(if .is_active then "✅ Activo" else "❌ Inactivo" end)",
            ""
        '
    fi
}

# Crear template desde archivo JSON
create_template() {
    local file="$1"
    
    if [ -z "$file" ]; then
        echo -e "${RED}Error: Especifica el archivo JSON del template${NC}"
        echo "Uso: $0 create-template template.json"
        return 1
    fi
    
    if [ ! -f "$file" ]; then
        echo -e "${RED}Error: Archivo $file no encontrado${NC}"
        return 1
    fi
    
    echo -e "${BLUE}Creando template desde $file...${NC}"
    
    local data=$(cat "$file")
    if response=$(api_request "POST" "/api/v1/notifications/templates" "$data"); then
        local template_id=$(echo "$response" | jq -r '.data.id')
        local template_name=$(echo "$response" | jq -r '.data.name')
        echo -e "${GREEN}✅ Template '$template_name' creado con ID: $template_id${NC}"
    fi
}

# Probar configuración SMTP
test_config() {
    echo -e "${BLUE}Probando configuración SMTP...${NC}"
    
    if api_request "POST" "/api/v1/notifications/admin/test-config" "" > /dev/null; then
        echo -e "${GREEN}✅ Configuración SMTP funciona correctamente${NC}"
    fi
}

# Monitoreo en tiempo real
monitor() {
    echo -e "${BLUE}Iniciando monitoreo en tiempo real...${NC}"
    echo "Presiona Ctrl+C para detener"
    echo ""
    
    while true; do
        clear
        echo "=== MONITOREO DEL SISTEMA DE NOTIFICACIONES ==="
        echo "Actualización: $(date)"
        echo ""
        
        check_status
        echo ""
        
        echo -e "${BLUE}Estadísticas de la última hora:${NC}"
        show_stats 0.04 # ~1 hora
        
        sleep 30
    done
}

# Función principal
main() {
    local command="$1"
    shift
    
    if [ -z "$command" ]; then
        show_help
        exit 0
    fi
    
    # Verificar dependencias
    check_dependencies
    
    case "$command" in
        "status")
            check_status
            ;;
        "stats")
            show_stats "$@"
            ;;
        "send-test")
            send_test_email "$@"
            ;;
        "process-pending")
            process_pending
            ;;
        "retry-failed")
            retry_failed
            ;;
        "cleanup")
            cleanup_old "$@"
            ;;
        "list-notifications"|"list")
            list_notifications
            ;;
        "list-templates"|"templates")
            list_templates
            ;;
        "create-template")
            create_template "$@"
            ;;
        "test-config")
            test_config
            ;;
        "monitor")
            monitor
            ;;
        "help"|"-h"|"--help")
            show_help
            ;;
        *)
            echo -e "${RED}Error: Comando '$command' no reconocido${NC}"
            echo ""
            show_help
            exit 1
            ;;
    esac
}

# Ejecutar función principal con todos los argumentos
main "$@"