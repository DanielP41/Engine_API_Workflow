package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"mime"
	"net"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// ================================
// IMPLEMENTACIÓN DEL SERVICIO SMTP
// ================================

// SMTPServiceImpl implementación concreta del servicio SMTP
type SMTPServiceImpl struct {
	config    *Config
	logger    *zap.Logger
	stats     *EmailStats
	statsMux  sync.RWMutex
	startTime time.Time

	// Pool de conexiones
	connPool    chan *smtpConnection
	connPoolMux sync.Mutex
	activeConns int

	// Rate limiting
	rateLimiter *rate.Limiter

	// Control de contexto
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Modo desarrollo
	devStorage []EmailMessage
	devMux     sync.RWMutex
}

// smtpConnection representa una conexión SMTP reutilizable
type smtpConnection struct {
	client    *smtp.Client
	conn      net.Conn
	createdAt time.Time
	lastUsed  time.Time
	inUse     bool
}

// NewSMTPService crea una nueva instancia del servicio SMTP
func NewSMTPService(config *Config) EmailService {
	if config == nil {
		config = NewDefaultConfig()
	}

	// Validar configuración
	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("invalid email config: %v", err))
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Configurar rate limiter
	rateLimiter := rate.NewLimiter(
		rate.Limit(config.Performance.RateLimit)/60, // por segundo
		config.Performance.BurstLimit,
	)

	service := &SMTPServiceImpl{
		config:      config,
		logger:      zap.NewNop(), // Se puede configurar después
		stats:       &EmailStats{},
		startTime:   time.Now(),
		connPool:    make(chan *smtpConnection, config.Performance.MaxConnections),
		rateLimiter: rateLimiter,
		ctx:         ctx,
		cancel:      cancel,
		devStorage:  make([]EmailMessage, 0),
	}

	// Inicializar pool de conexiones si no está en modo mock
	if !config.Development.MockMode {
		service.initConnectionPool()
	}

	// Iniciar worker de limpieza
	service.startCleanupWorker()

	return service
}

// SetLogger configura el logger del servicio
func (s *SMTPServiceImpl) SetLogger(logger *zap.Logger) {
	if logger != nil {
		s.logger = logger
	}
}

// ================================
// IMPLEMENTACIÓN DE LA INTERFAZ EmailService
// ================================

// Send implementa EmailService.Send
func (s *SMTPServiceImpl) Send(ctx context.Context, message *Message) error {
	// Convertir Message a EmailMessage
	emailMsg := message.ToEmailMessage()
	return s.SendEmail(ctx, emailMsg)
}

// SendWithID implementa EmailService.SendWithID
func (s *SMTPServiceImpl) SendWithID(ctx context.Context, message *Message) (string, error) {
	if err := s.Send(ctx, message); err != nil {
		return "", err
	}
	return message.ID, nil
}

// SendBatch implementa EmailService.SendBatch
func (s *SMTPServiceImpl) SendBatch(ctx context.Context, messages []*Message) error {
	emailMessages := make([]*EmailMessage, len(messages))
	for i, msg := range messages {
		emailMessages[i] = msg.ToEmailMessage()
	}
	return s.SendBatchEmails(ctx, emailMessages)
}

// TestConnection implementa EmailService.TestConnection
func (s *SMTPServiceImpl) TestConnection() error {
	return s.TestConnectionWithContext(context.Background())
}

// GetConfig implementa EmailService.GetConfig
func (s *SMTPServiceImpl) GetConfig() *Config {
	return s.config
}

// IsEnabled implementa EmailService.IsEnabled
func (s *SMTPServiceImpl) IsEnabled() bool {
	return !s.config.Development.MockMode
}

// GetStats implementa EmailService.GetStats
func (s *SMTPServiceImpl) GetStats() *EmailStats {
	return s.GetServiceStats()
}

// Close implementa EmailService.Close
func (s *SMTPServiceImpl) Close() error {
	return s.Shutdown(context.Background())
}

// ================================
// MÉTODOS PRINCIPALES
// ================================

// SendEmail envía un email
func (s *SMTPServiceImpl) SendEmail(ctx context.Context, email *EmailMessage) error {
	start := time.Now()

	// Validar mensaje
	if err := s.validateMessage(email); err != nil {
		s.incrementFailed()
		return fmt.Errorf("invalid email message: %w", err)
	}

	// Aplicar rate limiting
	if err := s.rateLimiter.Wait(ctx); err != nil {
		s.incrementFailed()
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Modo desarrollo
	if s.config.Development.MockMode {
		return s.handleMockEmail(email)
	}

	// Enviar email real
	err := s.sendEmailWithRetry(ctx, email)

	// Actualizar estadísticas
	s.updateStats(start, err)

	if err != nil {
		s.logger.Error("Failed to send email",
			zap.String("to", strings.Join(email.To, ",")),
			zap.String("subject", email.Subject),
			zap.Error(err))
		return err
	}

	s.logger.Info("Email sent successfully",
		zap.String("to", strings.Join(email.To, ",")),
		zap.String("subject", email.Subject),
		zap.Duration("duration", time.Since(start)))

	return nil
}

// SendSimpleEmail envía un email simple
func (s *SMTPServiceImpl) SendSimpleEmail(ctx context.Context, to []string, subject, body string, isHTML bool) error {
	email := &EmailMessage{
		To:      to,
		Subject: subject,
		Body:    body,
		IsHTML:  isHTML,
	}
	return s.SendEmail(ctx, email)
}

// SendTemplateEmail envía un email usando un template (placeholder)
func (s *SMTPServiceImpl) SendTemplateEmail(ctx context.Context, templateName string, to []string, data map[string]interface{}) error {
	// TODO: Implementar cuando tengamos el sistema de templates
	return fmt.Errorf("template emails not implemented yet")
}

// SendBatchEmails envía múltiples emails en lote
func (s *SMTPServiceImpl) SendBatchEmails(ctx context.Context, emails []*EmailMessage) error {
	if len(emails) == 0 {
		return nil
	}

	s.logger.Info("Starting batch email send", zap.Int("count", len(emails)))

	// Procesar en batches para evitar sobrecarga
	batchSize := s.config.Performance.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	var errors []error
	for i := 0; i < len(emails); i += batchSize {
		end := i + batchSize
		if end > len(emails) {
			end = len(emails)
		}

		batch := emails[i:end]
		if err := s.processBatch(ctx, batch); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("batch processing failed with %d errors: %v", len(errors), errors[0])
	}

	return nil
}

// ================================
// MÉTODOS DE GESTIÓN
// ================================

// TestConnectionWithContext prueba la conexión SMTP
func (s *SMTPServiceImpl) TestConnectionWithContext(ctx context.Context) error {
	if s.config.Development.MockMode {
		s.logger.Info("Test connection successful (mock mode)")
		return nil
	}

	conn, err := s.createConnection(ctx)
	if err != nil {
		return err
	}
	defer s.closeConnection(conn)

	s.logger.Info("Test connection successful")
	return nil
}

// GetServiceStats devuelve las estadísticas del servicio
func (s *SMTPServiceImpl) GetServiceStats() *EmailStats {
	s.statsMux.RLock()
	defer s.statsMux.RUnlock()

	stats := *s.stats
	stats.Uptime = time.Since(s.startTime)
	stats.ActiveConns = s.activeConns

	if s.config.Development.MockMode {
		s.devMux.RLock()
		stats.IdleConns = len(s.devStorage)
		s.devMux.RUnlock()
	}

	return &stats
}

// IsHealthy verifica si el servicio está saludable
func (s *SMTPServiceImpl) IsHealthy() bool {
	// Verificar configuración básica
	if s.config == nil {
		return false
	}

	// En modo mock, siempre saludable
	if s.config.Development.MockMode {
		return true
	}

	// Verificar conexión disponible
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.TestConnectionWithContext(ctx)
	return err == nil
}

// Shutdown cierra el servicio de forma elegante
func (s *SMTPServiceImpl) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down email service")

	// Cancelar contexto
	s.cancel()

	// Esperar que terminen las operaciones
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("Email service shutdown complete")
	case <-ctx.Done():
		s.logger.Warn("Email service shutdown timeout")
		return ctx.Err()
	}

	// Cerrar pool de conexiones
	s.closeConnectionPool()

	return nil
}

// ================================
// MÉTODOS INTERNOS
// ================================

// sendEmailWithRetry envía un email con reintentos automáticos
func (s *SMTPServiceImpl) sendEmailWithRetry(ctx context.Context, email *EmailMessage) error {
	var lastErr error

	for attempt := 1; attempt <= s.config.Retry.MaxAttempts; attempt++ {
		err := s.sendEmailOnce(ctx, email)
		if err == nil {
			return nil
		}

		lastErr = err

		// Verificar si el error es recuperable
		if !s.isRetryableError(err) {
			return err
		}

		// No reintentar en el último intento
		if attempt == s.config.Retry.MaxAttempts {
			break
		}

		// Calcular delay con backoff exponencial
		delay := s.calculateRetryDelay(attempt)

		s.logger.Warn("Email send failed, retrying",
			zap.Int("attempt", attempt),
			zap.Duration("delay", delay),
			zap.Error(err))

		// Esperar antes del siguiente intento
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continúa al siguiente intento
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", s.config.Retry.MaxAttempts, lastErr)
}

// sendEmailOnce envía un email una sola vez
func (s *SMTPServiceImpl) sendEmailOnce(ctx context.Context, email *EmailMessage) error {
	// Obtener conexión del pool
	conn, err := s.getConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer s.releaseConnection(conn)

	// Generar mensaje MIME
	message, err := s.buildMIMEMessage(email)
	if err != nil {
		return fmt.Errorf("failed to build message: %w", err)
	}

	// Configurar remitente
	from := email.From
	if from == "" {
		from = s.config.SMTP.DefaultFrom.Email
	}

	// Enviar email
	if err := conn.client.Mail(from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Configurar destinatarios
	allRecipients := append(email.To, email.CC...)
	allRecipients = append(allRecipients, email.BCC...)

	for _, recipient := range allRecipients {
		if err := conn.client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	// Enviar datos
	writer, err := conn.client.Data()
	if err != nil {
		return fmt.Errorf("failed to open data writer: %w", err)
	}

	if _, err := writer.Write(message); err != nil {
		writer.Close()
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	// Resetear para próximo uso
	if err := conn.client.Reset(); err != nil {
		// Log pero no fallar, la conexión puede seguir siendo útil
		s.logger.Warn("Failed to reset SMTP connection", zap.Error(err))
	}

	return nil
}

// buildMIMEMessage construye el mensaje MIME completo
func (s *SMTPServiceImpl) buildMIMEMessage(email *EmailMessage) ([]byte, error) {
	var buffer bytes.Buffer

	// Headers principales
	from := email.From
	fromName := email.FromName
	if from == "" {
		from = s.config.SMTP.DefaultFrom.Email
		fromName = s.config.SMTP.DefaultFrom.Name
	}

	// From header
	if fromName != "" {
		buffer.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, from))
	} else {
		buffer.WriteString(fmt.Sprintf("From: <%s>\r\n", from))
	}

	// To header
	buffer.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(email.To, ", ")))

	// CC header
	if len(email.CC) > 0 {
		buffer.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(email.CC, ", ")))
	}

	// Reply-To header
	if email.ReplyTo != "" {
		buffer.WriteString(fmt.Sprintf("Reply-To: %s\r\n", email.ReplyTo))
	}

	// Subject header
	buffer.WriteString(fmt.Sprintf("Subject: %s\r\n", s.encodeHeader(email.Subject)))

	// Message-ID
	messageID := email.MessageID
	if messageID == "" {
		messageID = s.generateMessageID()
	}
	buffer.WriteString(fmt.Sprintf("Message-ID: <%s>\r\n", messageID))

	// Date header
	buffer.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))

	// MIME headers
	buffer.WriteString("MIME-Version: 1.0\r\n")

	// Priority
	if email.Priority > 0 {
		priority := "3" // Normal por defecto
		switch email.Priority {
		case 1:
			priority = "1" // High
		case 5:
			priority = "5" // Low
		}
		buffer.WriteString(fmt.Sprintf("X-Priority: %s\r\n", priority))
	}

	// Headers personalizados
	for key, value := range email.Headers {
		buffer.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}

	// Headers del sistema
	for key, value := range s.config.Headers {
		buffer.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}

	// Determinar tipo de contenido
	if len(email.Attachments) > 0 {
		// Mensaje con adjuntos
		return s.buildMultipartMessage(&buffer, email)
	} else {
		// Mensaje simple
		return s.buildSimpleMessage(&buffer, email)
	}
}

// buildSimpleMessage construye un mensaje simple sin adjuntos
func (s *SMTPServiceImpl) buildSimpleMessage(buffer *bytes.Buffer, email *EmailMessage) ([]byte, error) {
	if email.IsHTML {
		buffer.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	} else {
		buffer.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	}
	buffer.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	buffer.WriteString("\r\n")

	// Codificar el cuerpo
	encodedBody := s.encodeQuotedPrintable(email.Body)
	buffer.WriteString(encodedBody)

	return buffer.Bytes(), nil
}

// buildMultipartMessage construye un mensaje con adjuntos
func (s *SMTPServiceImpl) buildMultipartMessage(buffer *bytes.Buffer, email *EmailMessage) ([]byte, error) {
	boundary := s.generateBoundary()

	buffer.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
	buffer.WriteString("\r\n")
	buffer.WriteString("This is a multi-part message in MIME format.\r\n")

	// Parte del cuerpo
	buffer.WriteString(fmt.Sprintf("\r\n--%s\r\n", boundary))
	if email.IsHTML {
		buffer.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	} else {
		buffer.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	}
	buffer.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	buffer.WriteString("\r\n")
	buffer.WriteString(s.encodeQuotedPrintable(email.Body))

	// Adjuntos
	for _, attachment := range email.Attachments {
		if err := s.addAttachment(buffer, boundary, &attachment); err != nil {
			return nil, fmt.Errorf("failed to add attachment %s: %w", attachment.Filename, err)
		}
	}

	// Boundary final
	buffer.WriteString(fmt.Sprintf("\r\n--%s--\r\n", boundary))

	return buffer.Bytes(), nil
}

// addAttachment añade un adjunto al mensaje
func (s *SMTPServiceImpl) addAttachment(buffer *bytes.Buffer, boundary string, attachment *Attachment) error {
	buffer.WriteString(fmt.Sprintf("\r\n--%s\r\n", boundary))

	// Determinar Content-Type
	contentType := attachment.ContentType
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(attachment.Filename))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	// Headers del adjunto
	if attachment.Inline && attachment.ContentID != "" {
		buffer.WriteString(fmt.Sprintf("Content-Type: %s\r\n", contentType))
		buffer.WriteString(fmt.Sprintf("Content-ID: <%s>\r\n", attachment.ContentID))
		buffer.WriteString("Content-Disposition: inline\r\n")
	} else {
		buffer.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", contentType, attachment.Filename))
		buffer.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", attachment.Filename))
	}

	buffer.WriteString("Content-Transfer-Encoding: base64\r\n")
	buffer.WriteString("\r\n")

	// Datos del adjunto
	var data []byte
	var err error

	if len(attachment.Data) > 0 {
		data = attachment.Data
	} else if attachment.FilePath != "" {
		data, err = os.ReadFile(attachment.FilePath)
		if err != nil {
			return fmt.Errorf("failed to read attachment file %s: %w", attachment.FilePath, err)
		}
	} else {
		return fmt.Errorf("attachment %s has no data or file path", attachment.Filename)
	}

	// Codificar en base64 con saltos de línea cada 76 caracteres
	encoded := base64.StdEncoding.EncodeToString(data)
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		buffer.WriteString(encoded[i:end])
		buffer.WriteString("\r\n")
	}

	return nil
}

// ================================
// GESTIÓN DE CONEXIONES
// ================================

// initConnectionPool inicializa el pool de conexiones
func (s *SMTPServiceImpl) initConnectionPool() {
	s.logger.Info("Initializing SMTP connection pool",
		zap.Int("max_connections", s.config.Performance.MaxConnections))

	// Pre-crear algunas conexiones
	initialConns := s.config.Performance.MaxIdleConns
	if initialConns > s.config.Performance.MaxConnections {
		initialConns = s.config.Performance.MaxConnections
	}

	for i := 0; i < initialConns; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
			defer cancel()

			if conn, err := s.createConnection(ctx); err == nil {
				select {
				case s.connPool <- conn:
				default:
					s.closeConnection(conn)
				}
			}
		}()
	}
}

// createConnection crea una nueva conexión SMTP
func (s *SMTPServiceImpl) createConnection(ctx context.Context) (*smtpConnection, error) {
	address := s.config.SMTP.GetAddress()

	// Conectar con timeout
	dialer := &net.Dialer{Timeout: s.config.SMTP.ConnectTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// Crear cliente SMTP
	client, err := smtp.NewClient(conn, s.config.SMTP.Host)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create SMTP client: %w", err)
	}

	// Configurar TLS si está habilitado
	if s.config.SMTP.EnableTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(s.config.SMTP.GetTLSConfig()); err != nil {
				client.Close()
				return nil, fmt.Errorf("STARTTLS failed: %w", err)
			}
		} else if s.config.SMTP.RequireSTARTTLS {
			client.Close()
			return nil, fmt.Errorf("STARTTLS required but not supported")
		}
	}

	// Autenticar si es necesario
	if s.config.SMTP.Username != "" && s.config.SMTP.Password != "" {
		auth := s.config.SMTP.GetSMTPAuth()
		if err := client.Auth(auth); err != nil {
			client.Close()
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
	}

	return &smtpConnection{
		client:    client,
		conn:      conn,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		inUse:     false,
	}, nil
}

// getConnection obtiene una conexión del pool
func (s *SMTPServiceImpl) getConnection(ctx context.Context) (*smtpConnection, error) {
	// Intentar obtener conexión existente
	select {
	case conn := <-s.connPool:
		if s.isConnectionValid(conn) {
			conn.inUse = true
			conn.lastUsed = time.Now()
			s.connPoolMux.Lock()
			s.activeConns++
			s.connPoolMux.Unlock()
			return conn, nil
		}
		s.closeConnection(conn)
	default:
		// Pool vacío, crear nueva conexión
	}

	// Crear nueva conexión
	conn, err := s.createConnection(ctx)
	if err != nil {
		return nil, err
	}

	conn.inUse = true
	s.connPoolMux.Lock()
	s.activeConns++
	s.connPoolMux.Unlock()

	return conn, nil
}

// releaseConnection libera una conexión de vuelta al pool
func (s *SMTPServiceImpl) releaseConnection(conn *smtpConnection) {
	if conn == nil {
		return
	}

	s.connPoolMux.Lock()
	s.activeConns--
	s.connPoolMux.Unlock()

	conn.inUse = false
	conn.lastUsed = time.Now()

	// Verificar si la conexión sigue siendo válida
	if !s.isConnectionValid(conn) {
		s.closeConnection(conn)
		return
	}

	// Devolver al pool si hay espacio
	select {
	case s.connPool <- conn:
		// Devuelta al pool exitosamente
	default:
		// Pool lleno, cerrar conexión
		s.closeConnection(conn)
	}
}

// isConnectionValid verifica si una conexión es válida
func (s *SMTPServiceImpl) isConnectionValid(conn *smtpConnection) bool {
	if conn == nil || conn.client == nil {
		return false
	}

	// Verificar timeout de conexión
	if time.Since(conn.lastUsed) > s.config.SMTP.IdleTimeout {
		return false
	}

	// Verificar que la conexión siga activa (NOOP)
	if err := conn.client.Noop(); err != nil {
		return false
	}

	return true
}

// closeConnection cierra una conexión
func (s *SMTPServiceImpl) closeConnection(conn *smtpConnection) {
	if conn != nil && conn.client != nil {
		conn.client.Quit()
		conn.client.Close()
	}
}

// closeConnectionPool cierra todas las conexiones del pool
func (s *SMTPServiceImpl) closeConnectionPool() {
	close(s.connPool)
	for conn := range s.connPool {
		s.closeConnection(conn)
	}
}

// ================================
// WORKERS Y TAREAS EN BACKGROUND
// ================================

// startCleanupWorker inicia el worker de limpieza de conexiones
func (s *SMTPServiceImpl) startCleanupWorker() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.cleanupConnections()
			}
		}
	}()
}

// cleanupConnections limpia conexiones inactivas
func (s *SMTPServiceImpl) cleanupConnections() {
	var activeConns []*smtpConnection

	// Drenar pool y verificar conexiones
	for {
		select {
		case conn := <-s.connPool:
			if s.isConnectionValid(conn) {
				activeConns = append(activeConns, conn)
			} else {
				s.closeConnection(conn)
			}
		default:
			// Pool vacío
			goto done
		}
	}

done:
	// Devolver conexiones válidas al pool
	for _, conn := range activeConns {
		select {
		case s.connPool <- conn:
		default:
			s.closeConnection(conn)
		}
	}

	s.logger.Debug("Connection pool cleanup completed",
		zap.Int("active_connections", len(activeConns)))
}

// ================================
// MÉTODOS DE UTILIDAD
// ================================

// validateMessage valida un mensaje de email
func (s *SMTPServiceImpl) validateMessage(email *EmailMessage) error {
	if len(email.To) == 0 {
		return fmt.Errorf("no recipients specified")
	}

	if len(email.To) > s.config.Security.MaxRecipients {
		return fmt.Errorf("too many recipients: %d (max %d)", len(email.To), s.config.Security.MaxRecipients)
	}

	if strings.TrimSpace(email.Subject) == "" {
		return fmt.Errorf("subject is required")
	}

	if strings.TrimSpace(email.Body) == "" {
		return fmt.Errorf("body is required")
	}

	// Validar destinatarios
	allEmails := append(email.To, email.CC...)
	allEmails = append(allEmails, email.BCC...)

	for _, addr := range allEmails {
		if !s.config.Security.IsEmailAllowed(addr) {
			return fmt.Errorf("email address not allowed: %s", addr)
		}
	}

	return nil
}

// processBatch procesa un lote de emails
func (s *SMTPServiceImpl) processBatch(ctx context.Context, emails []*EmailMessage) error {
	errChan := make(chan error, len(emails))
	semaphore := make(chan struct{}, s.config.Performance.ProcessingWorkers)

	// Procesar emails en paralelo
	for _, email := range emails {
		go func(e *EmailMessage) {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := s.SendEmail(ctx, e); err != nil {
				errChan <- err
			} else {
				errChan <- nil
			}
		}(email)
	}

	// Recoger resultados
	var errors []error
	for i := 0; i < len(emails); i++ {
		if err := <-errChan; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("batch failed with %d errors: %v", len(errors), errors[0])
	}

	return nil
}

// isRetryableError determina si un error es recuperable
func (s *SMTPServiceImpl) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Errores temporales conocidos
	retryableErrors := []string{
		"timeout",
		"connection",
		"temporary",
		"network",
		"dial",
		"read",
		"write",
		"broken pipe",
		"connection reset",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}

	// Verificar errores SMTP específicos
	if textErr, ok := err.(*textproto.Error); ok {
		// 4xx códigos son temporales
		return textErr.Code >= 400 && textErr.Code < 500
	}

	return false
}

// calculateRetryDelay calcula el delay para el próximo intento
func (s *SMTPServiceImpl) calculateRetryDelay(attempt int) time.Duration {
	delay := s.config.Retry.InitialDelay

	// Backoff exponencial
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * s.config.Retry.BackoffFactor)
		if delay > s.config.Retry.MaxDelay {
			delay = s.config.Retry.MaxDelay
			break
		}
	}

	// Añadir jitter si está habilitado
	if s.config.Retry.JitterEnabled {
		jitter := time.Duration(rand.Float64() * float64(delay) * 0.1)
		delay += jitter
	}

	return delay
}

// updateStats actualiza las estadísticas del servicio
func (s *SMTPServiceImpl) updateStats(start time.Time, err error) {
	s.statsMux.Lock()
	defer s.statsMux.Unlock()

	duration := time.Since(start)

	if err != nil {
		s.stats.TotalFailed++
		s.stats.LastError = err.Error()
	} else {
		s.stats.TotalSent++
		now := time.Now()
		s.stats.LastSent = &now
	}

	// Actualizar latencia promedio (simple moving average)
	if s.stats.AverageTime == 0 {
		s.stats.AverageTime = duration
	} else {
		s.stats.AverageTime = (s.stats.AverageTime + duration) / 2
	}
}

// incrementFailed incrementa el contador de emails fallidos
func (s *SMTPServiceImpl) incrementFailed() {
	s.statsMux.Lock()
	s.stats.TotalFailed++
	s.statsMux.Unlock()
}

// ================================
// MODO DESARROLLO
// ================================

// handleMockEmail maneja el envío de email en modo mock
func (s *SMTPServiceImpl) handleMockEmail(email *EmailMessage) error {
	s.devMux.Lock()
	defer s.devMux.Unlock()

	// Almacenar en memoria
	s.devStorage = append(s.devStorage, *email)

	// Log del email "enviado"
	s.logger.Info("Mock email sent",
		zap.String("to", strings.Join(email.To, ",")),
		zap.String("subject", email.Subject))

	// Guardar en archivo si está configurado
	if s.config.Development.SaveToFile && s.config.Development.FileStoragePath != "" {
		return s.saveEmailToFile(email)
	}

	return nil
}

// saveEmailToFile guarda el email en un archivo
func (s *SMTPServiceImpl) saveEmailToFile(email *EmailMessage) error {
	if err := os.MkdirAll(s.config.Development.FileStoragePath, 0755); err != nil {
		return err
	}

	filename := fmt.Sprintf("email_%d.txt", time.Now().Unix())
	filepath := filepath.Join(s.config.Development.FileStoragePath, filename)

	content := fmt.Sprintf("To: %s\nSubject: %s\nDate: %s\n\n%s",
		strings.Join(email.To, ", "),
		email.Subject,
		time.Now().Format(time.RFC3339),
		email.Body)

	return os.WriteFile(filepath, []byte(content), 0644)
}

// GetMockEmails devuelve los emails almacenados en modo mock
func (s *SMTPServiceImpl) GetMockEmails() []EmailMessage {
	if !s.config.Development.MockMode {
		return nil
	}

	s.devMux.RLock()
	defer s.devMux.RUnlock()

	emails := make([]EmailMessage, len(s.devStorage))
	copy(emails, s.devStorage)
	return emails
}

// ClearMockEmails limpia los emails almacenados en modo mock
func (s *SMTPServiceImpl) ClearMockEmails() {
	if !s.config.Development.MockMode {
		return
	}

	s.devMux.Lock()
	s.devStorage = s.devStorage[:0]
	s.devMux.Unlock()
}

// ================================
// FUNCIONES DE CODIFICACIÓN
// ================================

// encodeHeader codifica un header para MIME
func (s *SMTPServiceImpl) encodeHeader(text string) string {
	// Para simplicidad, usar encoding básico
	return text
}

// encodeQuotedPrintable codifica texto en quoted-printable
func (s *SMTPServiceImpl) encodeQuotedPrintable(text string) string {
	// Implementación simple de quoted-printable
	var buffer bytes.Buffer
	for i, r := range text {
		if r == '\n' {
			buffer.WriteString("\r\n")
		} else if r < 32 || r > 126 || r == '=' {
			buffer.WriteString(fmt.Sprintf("=%02X", r))
		} else {
			buffer.WriteRune(r)
		}

		// Límite de línea de 76 caracteres
		if i > 0 && i%70 == 0 {
			buffer.WriteString("=\r\n")
		}
	}
	return buffer.String()
}

// generateMessageID genera un Message-ID único
func (s *SMTPServiceImpl) generateMessageID() string {
	hostname := s.config.SMTP.Host
	if hostname == "" {
		hostname = "localhost"
	}
	return fmt.Sprintf("%d.%d@%s", time.Now().Unix(), rand.Int(), hostname)
}

// generateBoundary genera un boundary para mensajes multipart
func (s *SMTPServiceImpl) generateBoundary() string {
	return fmt.Sprintf("boundary_%d_%d", time.Now().Unix(), rand.Int())
}
