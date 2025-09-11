package handlers

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"                //framework backend para Golang
	"go.mongodb.org/mongo-driver/bson/primitive" //para el meanejo de json binario(BSON)

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/pkg/jwt"
)

type WebHandler struct {
	userRepo        repository.UserRepository
	workflowService services.WorkflowService
	logService      services.LogService
	authService     *services.AuthServiceImpl
	jwtService      jwt.JWTService
}

func NewWebHandler(
	userRepo repository.UserRepository,
	workflowService services.WorkflowService,
	logService services.LogService,
	authService *services.AuthServiceImpl,
	jwtService jwt.JWTService,
) *WebHandler {
	return &WebHandler{
		userRepo:        userRepo,
		workflowService: workflowService,
		logService:      logService,
		authService:     authService,
		jwtService:      jwtService,
	}
}

// Index - Página principal
func (h *WebHandler) Index(c *fiber.Ctx) error {
	// Verificar si el usuario ya está autenticado
	token := c.Cookies("auth_token")
	if token != "" {
		if _, err := h.jwtService.ValidateToken(token); err == nil {
			return c.Redirect("/dashboard")
		}
	}
	return c.Redirect("/login")
}

// ShowLogin muestra la página de login
func (h *WebHandler) ShowLogin(c *fiber.Ctx) error {
	// Si ya está autenticado, redirigir al dashboard
	token := c.Cookies("auth_token")
	if token != "" {
		if _, err := h.jwtService.ValidateToken(token); err == nil {
			return c.Redirect("/dashboard")
		}
	}

	return c.Render("login", fiber.Map{
		"Title": "Login - Engine API Workflow",
		"Error": c.Query("error"),
	})
}

// HandleLogin procesa el login desde el formulario web
func (h *WebHandler) HandleLogin(c *fiber.Ctx) error {
	email := strings.TrimSpace(c.FormValue("email"))
	password := c.FormValue("password")

	if email == "" || password == "" {
		return c.Redirect("/login?error=Email and password are required")
	}

	// Buscar usuario
	user, err := h.userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Redirect("/login?error=Invalid credentials")
	}

	// Verificar contraseña
	if err := h.authService.CheckPassword(password, user.Password); err != nil {
		return c.Redirect("/login?error=Invalid credentials")
	}

	// Verificar que el usuario esté activo
	if !user.IsActive {
		return c.Redirect("/login?error=Account is deactivated")
	}

	// Generar tokens
	tokens, err := h.authService.GenerateTokens(user.ID, user.Email, string(user.Role))
	if err != nil {
		return c.Redirect("/login?error=Login failed, please try again")
	}

	// Actualizar último login
	h.userRepo.UpdateLastLogin(c.Context(), user.ID)

	// Establecer cookie con el token
	c.Cookie(&fiber.Cookie{
		Name:     "auth_token",
		Value:    tokens.AccessToken,
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true,
		Secure:   false, // En producción debe ser true
		SameSite: "Lax",
	})

	return c.Redirect("/dashboard")
}

// ShowDashboard muestra el dashboard principal
func (h *WebHandler) ShowDashboard(c *fiber.Ctx) error {
	user := h.getUserFromContext(c)
	if user == nil {
		return c.Redirect("/login")
	}

	// Obtener estadísticas del sistema
	systemStats, _ := h.logService.GetSystemStats(c.Context())

	// Obtener workflows del usuario
	workflows, _, _ := h.workflowService.GetUserWorkflows(c.Context(), user.ID, repository.PaginationOptions{
		Page:     1,
		PageSize: 5,
		SortBy:   "updated_at",
	})

	// Obtener logs recientes del usuario
	logs, _, _ := h.logService.GetUserLogs(c.Context(), user.ID, repository.PaginationOptions{
		Page:     1,
		PageSize: 10,
		SortBy:   "created_at",
	})

	return c.Render("dashboard", fiber.Map{
		"Title":       "Dashboard - Engine API Workflow",
		"User":        user,
		"SystemStats": systemStats,
		"Workflows":   workflows,
		"RecentLogs":  logs,
		"IsAdmin":     user.Role == models.RoleAdmin,
	})
}

// ShowWorkflows muestra la lista de workflows
func (h *WebHandler) ShowWorkflows(c *fiber.Ctx) error {
	user := h.getUserFromContext(c)
	if user == nil {
		return c.Redirect("/login")
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize := 20
	search := c.Query("search", "")
	status := c.Query("status", "")

	var workflows []models.Workflow
	var total int64
	var err error

	// Construir filtros
	filters := repository.WorkflowSearchFilters{}
	if user.Role != models.RoleAdmin {
		filters.UserID = &user.ID
	}
	if search != "" {
		filters.Query = search
	}
	if status != "" {
		workflowStatus := models.WorkflowStatus(status)
		filters.Status = &workflowStatus
	}

	workflows, total, err = h.workflowService.Search(c.Context(), filters, repository.PaginationOptions{
		Page:     page,
		PageSize: pageSize,
		SortBy:   "updated_at",
	})

	if err != nil {
		return c.Status(500).Render("error", fiber.Map{
			"Title": "Error",
			"Error": "Error loading workflows: " + err.Error(),
		})
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return c.Render("workflows", fiber.Map{
		"Title":       "Workflows - Engine API Workflow",
		"User":        user,
		"Workflows":   workflows,
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"Total":       total,
		"Search":      search,
		"Status":      status,
		"IsAdmin":     user.Role == models.RoleAdmin,
	})
}

// ShowWorkflowDetail muestra los detalles de un workflow
func (h *WebHandler) ShowWorkflowDetail(c *fiber.Ctx) error {
	user := h.getUserFromContext(c)
	if user == nil {
		return c.Redirect("/login")
	}

	workflowID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(400).Render("error", fiber.Map{
			"Title": "Error",
			"Error": "Invalid workflow ID",
		})
	}

	workflow, err := h.workflowService.GetByID(c.Context(), workflowID)
	if err != nil {
		return c.Status(404).Render("error", fiber.Map{
			"Title": "Error",
			"Error": "Workflow not found",
		})
	}

	// Verificar permisos
	if user.Role != models.RoleAdmin && workflow.UserID != user.ID {
		return c.Status(403).Render("error", fiber.Map{
			"Title": "Error",
			"Error": "Access denied",
		})
	}

	// Obtener logs del workflow
	logs, _, _ := h.logService.GetWorkflowLogs(c.Context(), workflowID, repository.PaginationOptions{
		Page:     1,
		PageSize: 20,
		SortBy:   "created_at",
	})

	// Obtener estadísticas del workflow
	stats, _ := h.logService.GetWorkflowStats(c.Context(), workflowID, 30)

	// Convertir steps a JSON para el editor
	stepsJSON, _ := json.Marshal(workflow.Steps)
	triggersJSON, _ := json.Marshal(workflow.Triggers)

	return c.Render("workflow-detail", fiber.Map{
		"Title":        "Workflow: " + workflow.Name,
		"User":         user,
		"Workflow":     workflow,
		"Logs":         logs,
		"Stats":        stats,
		"StepsJSON":    string(stepsJSON),
		"TriggersJSON": string(triggersJSON),
		"IsAdmin":      user.Role == models.RoleAdmin,
		"IsOwner":      workflow.UserID == user.ID,
	})
}

// ShowWorkflowCreate muestra el formulario para crear workflow
func (h *WebHandler) ShowWorkflowCreate(c *fiber.Ctx) error {
	user := h.getUserFromContext(c)
	if user == nil {
		return c.Redirect("/login")
	}

	return c.Render("workflow-create", fiber.Map{
		"Title":   "Create Workflow - Engine API Workflow",
		"User":    user,
		"IsAdmin": user.Role == models.RoleAdmin,
	})
}

// ShowLogs muestra la lista de logs
func (h *WebHandler) ShowLogs(c *fiber.Ctx) error {
	user := h.getUserFromContext(c)
	if user == nil {
		return c.Redirect("/login")
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize := 50
	workflowID := c.Query("workflow_id", "")
	status := c.Query("status", "")

	// Construir filtros
	filter := repository.LogSearchFilter{}
	if user.Role != models.RoleAdmin {
		filter.UserID = &user.ID
	}
	if workflowID != "" {
		if objID, err := primitive.ObjectIDFromHex(workflowID); err == nil {
			filter.WorkflowID = &objID
		}
	}
	if status != "" {
		filter.Status = &status
	}

	logs, total, err := h.logService.SearchLogs(c.Context(), filter, repository.PaginationOptions{
		Page:     page,
		PageSize: pageSize,
		SortBy:   "created_at",
	})

	if err != nil {
		return c.Status(500).Render("error", fiber.Map{
			"Title": "Error",
			"Error": "Error loading logs: " + err.Error(),
		})
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	// Obtener lista de workflows para filtro
	userWorkflows, _, _ := h.workflowService.GetUserWorkflows(c.Context(), user.ID, repository.PaginationOptions{
		Page:     1,
		PageSize: 100,
		SortBy:   "name",
	})

	return c.Render("logs", fiber.Map{
		"Title":         "Logs - Engine API Workflow",
		"User":          user,
		"Logs":          logs,
		"CurrentPage":   page,
		"TotalPages":    totalPages,
		"Total":         total,
		"WorkflowID":    workflowID,
		"Status":        status,
		"UserWorkflows": userWorkflows,
		"IsAdmin":       user.Role == models.RoleAdmin,
	})
}

// ShowUsers muestra la lista de usuarios (solo admins)
func (h *WebHandler) ShowUsers(c *fiber.Ctx) error {
	user := h.getUserFromContext(c)
	if user == nil {
		return c.Redirect("/login")
	}

	if user.Role != models.RoleAdmin {
		return c.Status(403).Render("error", fiber.Map{
			"Title": "Error",
			"Error": "Admin access required",
		})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize := 20

	userList, err := h.userRepo.List(c.Context(), page, pageSize)
	if err != nil {
		return c.Status(500).Render("error", fiber.Map{
			"Title": "Error",
			"Error": "Error loading users: " + err.Error(),
		})
	}

	return c.Render("users", fiber.Map{
		"Title":       "Users - Engine API Workflow",
		"User":        user,
		"Users":       userList.Users,
		"CurrentPage": page,
		"TotalPages":  userList.TotalPages,
		"Total":       userList.Total,
		"IsAdmin":     true,
	})
}

// Logout cierra la sesión
func (h *WebHandler) Logout(c *fiber.Ctx) error {
	// Limpiar cookie
	c.Cookie(&fiber.Cookie{
		Name:     "auth_token",
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		HTTPOnly: true,
	})

	return c.Redirect("/login")
}

// WebAuthMiddleware middleware para autenticación web
func (h *WebHandler) WebAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Cookies("auth_token")
		if token == "" {
			return c.Redirect("/login")
		}

		claims, err := h.jwtService.ValidateToken(token)
		if err != nil {
			// Token inválido, limpiar cookie y redirigir
			c.Cookie(&fiber.Cookie{
				Name:     "auth_token",
				Value:    "",
				Expires:  time.Now().Add(-time.Hour),
				HTTPOnly: true,
			})
			return c.Redirect("/login")
		}

		// Obtener usuario completo
		userID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			return c.Redirect("/login")
		}

		user, err := h.userRepo.GetByID(c.Context(), userID)
		if err != nil || !user.IsActive {
			return c.Redirect("/login")
		}

		// Almacenar en contexto
		c.Locals("user", user)
		c.Locals("userID", user.ID)
		c.Locals("userRole", user.Role)
		c.Locals("isAdmin", user.Role == models.RoleAdmin)

		return c.Next()
	}
}

// Helper methods
func (h *WebHandler) getUserFromContext(c *fiber.Ctx) *models.User {
	user := c.Locals("user")
	if user == nil {
		return nil
	}
	if u, ok := user.(*models.User); ok {
		return u
	}
	return nil
}

func (h *WebHandler) getUserIDFromContext(c *fiber.Ctx) primitive.ObjectID {
	user := h.getUserFromContext(c)
	if user != nil {
		return user.ID
	}
	return primitive.NilObjectID
}
