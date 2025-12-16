# Engine API Workflow Architecture

## Overview
Engine API Workflow is a robust, scalable backend solution designed to automate business processes through configurable workflows. It leverages a microservices-ready architecture using Go, MongoDB, Redis, and Containerization.

## System Architecture

```mermaid
graph TD
    Client[Client Apps/Web] -->|REST API| LB[Load Balancer]
    LB --> API[Engine API (Go/Fiber)]
    
    subgraph Core Services
        API --> Auth[Auth Service]
        API --> Workflow[Workflow Service]
        API --> Dashboard[Dashboard Service]
        Workflow --> Queue[Redis Queue]
    end
    
    subgraph Worker Ecosystem
        Queue --> Worker[Worker Engine]
        Worker --> Executors[Action Executors]
        Executors --> DB_Exec[Database Executor]
        Executors --> API_Exec[HTTP Executor]
        Executors --> Email_Exec[Email Executor]
    end
    
    subgraph Data Layer
        Auth --> Mongo[(MongoDB)]
        Workflow --> Mongo
        Dashboard --> Redis[(Redis Cache)]
    end
    
    Worker --> Mongo
```

## Key Components

### 1. API Layer (cmd/api)
- Built with **Fiber** for high performance.
- Handles HTTP requests, validation, and routing.
- middleware: Auth (JWT), CORS, Rate Limiting, Logger.

### 2. Service Layer (internal/services)
- Contains business logic.
- **WorkflowService**: Manages workflow definitions and state.
- **AuthService**: Handles user registration and authentication.
- **DashboardService**: Aggregates metrics for UI.
- **BackupService**: Manages system backups.

### 3. Data Access Layer (internal/repository)
- **MongoDB**: Primary storage for Users, Workflows, Logs.
- **Redis**: Used for:
    - Task Queue (Workflows)
    - Caching (Dashboard stats)
    - Pub/Sub (Real-time updates)

### 4. Worker Engine (internal/worker)
- Asynchronous task processing.
- Pulls tasks from Redis Queue.
- **Executors**:
    - `HTTP`: Makes external API calls.
    - `Database`: Executes safe DB operations.
    - `Email`: Sends notifications.
    - `Transform`: Manipulates data between steps.

## Data Flow
1. **Trigger**: User/Webhook initiates a Workflow.
2. **Queueing**: Workflow engine resolves the first step and pushes it to Redis.
3. **Processing**: Worker picks up the task.
4. **Execution**: Appropriate Executor runs the logic.
5. **Transition**: Result is analyzed; next step is determined and queued.
6. **Completion**: Workflow finishes or fails; logs are saved to MongoDB.

## Security
- **JWT**: Stateless authentication with Access/Refresh tokens.
- **RBAC**: Role-based access control (Admin/User).
- **Encryption**: Passwords hashed with bcrypt.
