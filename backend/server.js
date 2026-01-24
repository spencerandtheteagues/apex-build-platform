/**
 * APEX.BUILD Enterprise Backend
 * Production-grade Node.js server for AI-powered development platform
 */

const express = require('express');
const cors = require('cors');
const helmet = require('helmet');
const morgan = require('morgan');
const { v4: uuidv4 } = require('uuid');
const WebSocket = require('ws');
const http = require('http');
const fs = require('fs-extra');
const path = require('path');
const archiver = require('archiver');
const chokidar = require('chokidar');
require('dotenv').config();

// AI SDK imports
const Anthropic = require('anthropic');
const OpenAI = require('openai');

// Initialize Express app
const app = express();
const server = http.createServer(app);

// Configuration
const PORT = process.env.PORT || 3001;
const PROJECTS_DIR = path.join(__dirname, 'projects');
const GENERATED_APPS_DIR = path.join(__dirname, 'generated-apps');

// Ensure directories exist
fs.ensureDirSync(PROJECTS_DIR);
fs.ensureDirSync(GENERATED_APPS_DIR);

// AI Clients
const anthropic = new Anthropic({
  apiKey: process.env.ANTHROPIC_API_KEY || 'sk-ant-api03-mock-key-for-testing'
});

const openai = new OpenAI({
  apiKey: process.env.OPENAI_API_KEY || 'sk-mock-key-for-testing'
});

// In-memory storage (would be database in production)
const projects = new Map();
const activeConnections = new Map();

// Middleware
app.use(helmet({
  contentSecurityPolicy: {
    directives: {
      defaultSrc: ["'self'"],
      scriptSrc: ["'self'", "'unsafe-inline'", "'unsafe-eval'"],
      styleSrc: ["'self'", "'unsafe-inline'"],
      imgSrc: ["'self'", "data:", "https:"],
      connectSrc: ["'self'", "ws:", "wss:"]
    }
  }
}));

app.use(cors({
  origin: process.env.NODE_ENV === 'production'
    ? ['https://apex.build', 'https://www.apex.build']
    : ['http://localhost:8000', 'http://localhost:3000'],
  credentials: true
}));

app.use(morgan('combined'));
app.use(express.json({ limit: '50mb' }));
app.use(express.urlencoded({ extended: true, limit: '50mb' }));

// Serve generated apps statically
app.use('/apps', express.static(GENERATED_APPS_DIR));

// WebSocket Setup for Real-time Collaboration
const wss = new WebSocket.Server({ server });

wss.on('connection', (ws, req) => {
  const connectionId = uuidv4();
  activeConnections.set(connectionId, ws);

  console.log(`✅ WebSocket connection established: ${connectionId}`);

  ws.on('message', async (message) => {
    try {
      const data = JSON.parse(message);

      switch (data.type) {
        case 'join_project':
          ws.projectId = data.projectId;
          break;

        case 'code_change':
          // Broadcast code changes to other users in the same project
          broadcastToProject(data.projectId, {
            type: 'code_update',
            file: data.file,
            content: data.content,
            userId: connectionId
          }, connectionId);
          break;

        case 'cursor_position':
          // Broadcast cursor position for collaborative editing
          broadcastToProject(data.projectId, {
            type: 'cursor_update',
            line: data.line,
            column: data.column,
            userId: connectionId
          }, connectionId);
          break;
      }
    } catch (error) {
      console.error('WebSocket message error:', error);
    }
  });

  ws.on('close', () => {
    activeConnections.delete(connectionId);
    console.log(`❌ WebSocket connection closed: ${connectionId}`);
  });
});

function broadcastToProject(projectId, message, excludeConnectionId = null) {
  activeConnections.forEach((ws, connectionId) => {
    if (ws.projectId === projectId && connectionId !== excludeConnectionId && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(message));
    }
  });
}

// AI Generation Service
class AIOrchestrator {
  constructor() {
    this.models = {
      claude: {
        name: 'Claude Opus 4.5',
        client: anthropic,
        specialty: 'reasoning'
      },
      gpt: {
        name: 'GPT-5 Turbo',
        client: openai,
        specialty: 'speed'
      },
      gemini: {
        name: 'Gemini Pro 3',
        client: null, // Would be Google AI client
        specialty: 'multimodal'
      }
    };
  }

  async generateApp(prompt, selectedAI = 'claude') {
    console.log(`🤖 Generating app with ${selectedAI}: "${prompt}"`);

    try {
      const appSpec = await this.analyzePrompt(prompt, selectedAI);
      const files = await this.generateFiles(appSpec, selectedAI);
      const projectId = uuidv4();

      // Create project directory
      const projectPath = path.join(GENERATED_APPS_DIR, projectId);
      fs.ensureDirSync(projectPath);

      // Write files to disk
      for (const [filename, content] of Object.entries(files)) {
        await fs.writeFile(path.join(projectPath, filename), content);
      }

      // Create project metadata
      const project = {
        id: projectId,
        name: appSpec.name,
        description: prompt,
        aiModel: selectedAI,
        files: files,
        createdAt: new Date().toISOString(),
        url: `/apps/${projectId}/index.html`
      };

      projects.set(projectId, project);

      return project;

    } catch (error) {
      console.error('App generation error:', error);
      throw new Error(`Failed to generate app: ${error.message}`);
    }
  }

  async analyzePrompt(prompt, aiModel) {
    const analysisPrompt = `Analyze this app request and return a JSON spec:
    "${prompt}"

    Return JSON with:
    {
      "name": "App name",
      "type": "web app type (spa, landing, dashboard, etc)",
      "features": ["feature1", "feature2"],
      "styling": "styling approach",
      "complexity": "simple|medium|complex"
    }`;

    if (aiModel === 'claude') {
      // Mock Claude response for testing
      return {
        name: prompt.slice(0, 30),
        type: "spa",
        features: ["responsive", "interactive"],
        styling: "modern",
        complexity: "medium"
      };
    }

    // Would implement actual AI calls here
    return {
      name: prompt.slice(0, 30),
      type: "spa",
      features: ["responsive", "interactive"],
      styling: "modern",
      complexity: "medium"
    };
  }

  async generateFiles(appSpec, aiModel) {
    const files = {};

    // Generate HTML
    files['index.html'] = this.generateHTML(appSpec);

    // Generate CSS
    files['style.css'] = this.generateCSS(appSpec);

    // Generate JavaScript
    files['script.js'] = this.generateJS(appSpec);

    // Generate README
    files['README.md'] = this.generateReadme(appSpec);

    return files;
  }

  generateHTML(appSpec) {
    return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${appSpec.name}</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <div class="container">
        <header class="header">
            <h1>${appSpec.name}</h1>
            <p>Generated by APEX.BUILD AI</p>
        </header>

        <main class="main-content">
            <section class="hero">
                <h2>Welcome to ${appSpec.name}</h2>
                <p>This app was generated based on your description.</p>
                <button class="cta-button" onclick="handleInteraction()">Get Started</button>
            </section>

            <section class="features">
                <h3>Features</h3>
                <div class="feature-grid">
                    ${appSpec.features.map(feature =>
                      `<div class="feature-card">
                        <h4>${feature}</h4>
                        <p>Advanced ${feature} functionality</p>
                      </div>`
                    ).join('')}
                </div>
            </section>

            <section class="interactive">
                <h3>Try It Out</h3>
                <div class="demo-area">
                    <input type="text" id="demo-input" placeholder="Enter some text...">
                    <button onclick="processInput()">Process</button>
                    <div id="demo-output"></div>
                </div>
            </section>
        </main>

        <footer class="footer">
            <p>Powered by APEX.BUILD Enterprise Platform</p>
        </footer>
    </div>

    <script src="script.js"></script>
</body>
</html>`;
  }

  generateCSS(appSpec) {
    return `/* ${appSpec.name} - Generated by APEX.BUILD */

:root {
    --primary: #667eea;
    --secondary: #764ba2;
    --accent: #f093fb;
    --text: #333;
    --background: #f8f9fa;
    --surface: #ffffff;
    --border: #e9ecef;
}

* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    background: linear-gradient(135deg, var(--primary), var(--secondary));
    color: var(--text);
    min-height: 100vh;
}

.container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 20px;
}

.header {
    text-align: center;
    padding: 60px 0;
    color: white;
}

.header h1 {
    font-size: 3rem;
    font-weight: 800;
    margin-bottom: 10px;
}

.header p {
    font-size: 1.2rem;
    opacity: 0.9;
}

.main-content {
    background: var(--surface);
    border-radius: 20px;
    padding: 40px;
    box-shadow: 0 20px 40px rgba(0,0,0,0.1);
    margin-bottom: 40px;
}

.hero {
    text-align: center;
    margin-bottom: 60px;
}

.hero h2 {
    font-size: 2.5rem;
    color: var(--primary);
    margin-bottom: 20px;
}

.hero p {
    font-size: 1.2rem;
    color: #666;
    margin-bottom: 30px;
}

.cta-button {
    background: linear-gradient(135deg, var(--primary), var(--secondary));
    color: white;
    border: none;
    padding: 15px 40px;
    border-radius: 50px;
    font-size: 1.1rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.3s ease;
    box-shadow: 0 10px 30px rgba(102, 126, 234, 0.4);
}

.cta-button:hover {
    transform: translateY(-3px);
    box-shadow: 0 15px 40px rgba(102, 126, 234, 0.6);
}

.features {
    margin-bottom: 60px;
}

.features h3 {
    font-size: 2rem;
    color: var(--primary);
    text-align: center;
    margin-bottom: 40px;
}

.feature-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
    gap: 30px;
}

.feature-card {
    background: var(--background);
    padding: 30px;
    border-radius: 15px;
    text-align: center;
    transition: transform 0.3s ease;
    border: 2px solid transparent;
}

.feature-card:hover {
    transform: translateY(-5px);
    border-color: var(--primary);
}

.feature-card h4 {
    color: var(--primary);
    margin-bottom: 15px;
    text-transform: capitalize;
}

.interactive {
    text-align: center;
}

.interactive h3 {
    font-size: 2rem;
    color: var(--primary);
    margin-bottom: 30px;
}

.demo-area {
    background: var(--background);
    padding: 40px;
    border-radius: 15px;
    max-width: 500px;
    margin: 0 auto;
}

.demo-area input {
    width: 100%;
    padding: 15px;
    border: 2px solid var(--border);
    border-radius: 10px;
    font-size: 1rem;
    margin-bottom: 20px;
    outline: none;
    transition: border-color 0.3s ease;
}

.demo-area input:focus {
    border-color: var(--primary);
}

.demo-area button {
    background: var(--primary);
    color: white;
    border: none;
    padding: 15px 30px;
    border-radius: 10px;
    font-size: 1rem;
    cursor: pointer;
    transition: background 0.3s ease;
}

.demo-area button:hover {
    background: var(--secondary);
}

#demo-output {
    margin-top: 20px;
    padding: 20px;
    background: white;
    border-radius: 10px;
    min-height: 60px;
    border: 2px dashed var(--border);
    color: #666;
}

.footer {
    text-align: center;
    color: white;
    opacity: 0.8;
    padding: 20px;
}

@media (max-width: 768px) {
    .container {
        padding: 10px;
    }

    .header h1 {
        font-size: 2rem;
    }

    .hero h2 {
        font-size: 1.8rem;
    }

    .main-content {
        padding: 20px;
    }

    .feature-grid {
        grid-template-columns: 1fr;
    }
}`;
  }

  generateJS(appSpec) {
    return `// ${appSpec.name} - Generated by APEX.BUILD

// App initialization
document.addEventListener('DOMContentLoaded', function() {
    console.log('🚀 ${appSpec.name} loaded successfully!');
    initializeApp();
});

function initializeApp() {
    // Add smooth scrolling
    document.querySelectorAll('a[href^="#"]').forEach(anchor => {
        anchor.addEventListener('click', function (e) {
            e.preventDefault();
            const target = document.querySelector(this.getAttribute('href'));
            if (target) {
                target.scrollIntoView({
                    behavior: 'smooth'
                });
            }
        });
    });

    // Add loading states to buttons
    document.querySelectorAll('button').forEach(button => {
        button.addEventListener('click', function() {
            this.style.transform = 'scale(0.95)';
            setTimeout(() => {
                this.style.transform = '';
            }, 150);
        });
    });

    // Initialize demo functionality
    setupDemo();
}

function handleInteraction() {
    const button = event.target;
    const originalText = button.textContent;

    button.textContent = 'Loading...';
    button.disabled = true;

    // Simulate some processing
    setTimeout(() => {
        button.textContent = '✅ Started!';

        setTimeout(() => {
            button.textContent = originalText;
            button.disabled = false;
        }, 2000);
    }, 1000);

    // Add some visual feedback
    createParticleEffect(button);
}

function processInput() {
    const input = document.getElementById('demo-input');
    const output = document.getElementById('demo-output');
    const value = input.value.trim();

    if (!value) {
        output.innerHTML = '<p style="color: #ff6b6b;">Please enter some text!</p>';
        return;
    }

    output.innerHTML = '<p>Processing...</p>';

    // Simulate processing
    setTimeout(() => {
        const processed = value
            .split('')
            .reverse()
            .join('')
            .toUpperCase();

        output.innerHTML = \`
            <div style="text-align: left;">
                <strong>Original:</strong> "\${value}"<br>
                <strong>Processed:</strong> "\${processed}"<br>
                <strong>Length:</strong> \${value.length} characters<br>
                <strong>Words:</strong> \${value.split(' ').length} words
            </div>
        \`;
    }, 800);
}

function createParticleEffect(element) {
    const rect = element.getBoundingClientRect();
    const centerX = rect.left + rect.width / 2;
    const centerY = rect.top + rect.height / 2;

    for (let i = 0; i < 12; i++) {
        createParticle(centerX, centerY);
    }
}

function createParticle(x, y) {
    const particle = document.createElement('div');
    particle.style.cssText = \`
        position: fixed;
        width: 6px;
        height: 6px;
        background: linear-gradient(45deg, #667eea, #764ba2);
        border-radius: 50%;
        pointer-events: none;
        z-index: 9999;
        left: \${x}px;
        top: \${y}px;
    \`;

    document.body.appendChild(particle);

    const angle = Math.random() * Math.PI * 2;
    const velocity = 3 + Math.random() * 4;
    const gravity = 0.15;
    let vx = Math.cos(angle) * velocity;
    let vy = Math.sin(angle) * velocity;
    let life = 60;

    function animate() {
        if (life <= 0) {
            particle.remove();
            return;
        }

        x += vx;
        y += vy;
        vy += gravity;
        life--;

        particle.style.left = x + 'px';
        particle.style.top = y + 'px';
        particle.style.opacity = life / 60;

        requestAnimationFrame(animate);
    }

    animate();
}

// Advanced features
class AppAnalytics {
    constructor() {
        this.events = [];
    }

    track(event, data = {}) {
        const eventData = {
            event,
            data,
            timestamp: new Date().toISOString(),
            url: window.location.href
        };

        this.events.push(eventData);
        console.log('📊 Analytics:', eventData);
    }

    getStats() {
        return {
            totalEvents: this.events.length,
            uniqueEvents: [...new Set(this.events.map(e => e.event))],
            sessionLength: this.events.length > 0
                ? Date.now() - new Date(this.events[0].timestamp).getTime()
                : 0
        };
    }
}

// Initialize analytics
const analytics = new AppAnalytics();

// Track page load
analytics.track('page_load', {
    app: '${appSpec.name}',
    features: ${JSON.stringify(appSpec.features)}
});

// Track interactions
document.addEventListener('click', (e) => {
    if (e.target.tagName === 'BUTTON') {
        analytics.track('button_click', {
            button: e.target.textContent,
            location: e.target.className
        });
    }
});

console.log('✅ ${appSpec.name} fully initialized with analytics');`;
  }

  generateReadme(appSpec) {
    return `# ${appSpec.name}

Generated by APEX.BUILD Enterprise Platform

## Description
${appSpec.name} is a ${appSpec.type} application with ${appSpec.complexity} complexity.

## Features
${appSpec.features.map(f => `- ${f}`).join('\n')}

## Technology Stack
- HTML5 with semantic markup
- CSS3 with modern features (Grid, Flexbox, Custom Properties)
- Vanilla JavaScript with ES6+ features
- Responsive design for all devices

## File Structure
\`\`\`
├── index.html          # Main HTML file
├── style.css           # Stylesheet
├── script.js           # JavaScript functionality
└── README.md           # This file
\`\`\`

## Development
This app is ready to run in any modern web browser. Simply open \`index.html\` in your browser.

## Deployment
Upload all files to any web server or static hosting platform.

## Generated by APEX.BUILD
This application was automatically generated using advanced AI models and can be further customized as needed.

---
Created with ❤️ by APEX.BUILD Enterprise Platform
`;
  }
}

// Initialize AI Orchestrator
const aiOrchestrator = new AIOrchestrator();

// API Routes

// Health check
app.get('/health', (req, res) => {
  res.json({
    status: 'healthy',
    service: 'APEX.BUILD Backend',
    version: '2.0.0',
    timestamp: new Date().toISOString(),
    uptime: process.uptime()
  });
});

// Generate app from prompt
app.post('/api/v1/generate', async (req, res) => {
  try {
    const { prompt, aiModel = 'claude' } = req.body;

    if (!prompt || prompt.trim().length === 0) {
      return res.status(400).json({
        error: 'Prompt is required',
        code: 'MISSING_PROMPT'
      });
    }

    console.log(`🚀 Generating app: "${prompt}" with ${aiModel}`);

    const project = await aiOrchestrator.generateApp(prompt, aiModel);

    res.json({
      success: true,
      project: project,
      message: 'App generated successfully'
    });

  } catch (error) {
    console.error('Generation error:', error);
    res.status(500).json({
      error: 'Failed to generate app',
      message: error.message,
      code: 'GENERATION_ERROR'
    });
  }
});

// Get all projects
app.get('/api/v1/projects', (req, res) => {
  const allProjects = Array.from(projects.values()).sort((a, b) =>
    new Date(b.createdAt) - new Date(a.createdAt)
  );

  res.json({
    success: true,
    projects: allProjects,
    total: allProjects.length
  });
});

// Get specific project
app.get('/api/v1/projects/:id', (req, res) => {
  const project = projects.get(req.params.id);

  if (!project) {
    return res.status(404).json({
      error: 'Project not found',
      code: 'PROJECT_NOT_FOUND'
    });
  }

  res.json({
    success: true,
    project: project
  });
});

// Update project files
app.put('/api/v1/projects/:id/files/:filename', async (req, res) => {
  try {
    const { id, filename } = req.params;
    const { content } = req.body;

    const project = projects.get(id);
    if (!project) {
      return res.status(404).json({
        error: 'Project not found',
        code: 'PROJECT_NOT_FOUND'
      });
    }

    // Update in memory
    project.files[filename] = content;

    // Write to disk
    const filePath = path.join(GENERATED_APPS_DIR, id, filename);
    await fs.writeFile(filePath, content);

    // Broadcast change to WebSocket clients
    broadcastToProject(id, {
      type: 'file_updated',
      filename,
      content
    });

    res.json({
      success: true,
      message: 'File updated successfully'
    });

  } catch (error) {
    console.error('File update error:', error);
    res.status(500).json({
      error: 'Failed to update file',
      message: error.message
    });
  }
});

// Delete project
app.delete('/api/v1/projects/:id', async (req, res) => {
  try {
    const { id } = req.params;

    const project = projects.get(id);
    if (!project) {
      return res.status(404).json({
        error: 'Project not found',
        code: 'PROJECT_NOT_FOUND'
      });
    }

    // Remove from memory
    projects.delete(id);

    // Remove from disk
    const projectPath = path.join(GENERATED_APPS_DIR, id);
    await fs.remove(projectPath);

    res.json({
      success: true,
      message: 'Project deleted successfully'
    });

  } catch (error) {
    console.error('Project deletion error:', error);
    res.status(500).json({
      error: 'Failed to delete project',
      message: error.message
    });
  }
});

// Download project as ZIP
app.get('/api/v1/projects/:id/download', async (req, res) => {
  try {
    const { id } = req.params;

    const project = projects.get(id);
    if (!project) {
      return res.status(404).json({
        error: 'Project not found'
      });
    }

    const projectPath = path.join(GENERATED_APPS_DIR, id);

    res.setHeader('Content-Type', 'application/zip');
    res.setHeader('Content-Disposition', `attachment; filename="${project.name}.zip"`);

    const archive = archiver('zip', { zlib: { level: 9 } });
    archive.pipe(res);
    archive.directory(projectPath, false);
    await archive.finalize();

  } catch (error) {
    console.error('Download error:', error);
    res.status(500).json({
      error: 'Failed to download project'
    });
  }
});

// AI Models status
app.get('/api/v1/ai/status', (req, res) => {
  res.json({
    success: true,
    models: {
      claude: {
        name: 'Claude Opus 4.5',
        status: process.env.ANTHROPIC_API_KEY ? 'available' : 'api_key_missing',
        specialty: 'Advanced reasoning and code generation'
      },
      gpt: {
        name: 'GPT-5 Turbo',
        status: process.env.OPENAI_API_KEY ? 'available' : 'api_key_missing',
        specialty: 'Lightning-fast development'
      },
      gemini: {
        name: 'Gemini Pro 3',
        status: 'coming_soon',
        specialty: 'Multimodal app development'
      }
    }
  });
});

// Error handling middleware
app.use((error, req, res, next) => {
  console.error('Unhandled error:', error);
  res.status(500).json({
    error: 'Internal server error',
    message: process.env.NODE_ENV === 'development' ? error.message : 'Something went wrong'
  });
});

// 404 handler
app.use((req, res) => {
  res.status(404).json({
    error: 'Route not found',
    path: req.path,
    method: req.method
  });
});

// Start server
server.listen(PORT, () => {
  console.log('');
  console.log('🚀 APEX.BUILD Enterprise Backend Server Started');
  console.log('===========================================');
  console.log(`📡 Server: http://localhost:${PORT}`);
  console.log(`🤖 AI Models: ${Object.keys(aiOrchestrator.models).join(', ')}`);
  console.log(`📁 Projects: ${PROJECTS_DIR}`);
  console.log(`🌐 Generated Apps: ${GENERATED_APPS_DIR}`);
  console.log(`🔌 WebSocket: Ready for real-time collaboration`);
  console.log('===========================================');
  console.log('');
});

// Graceful shutdown
process.on('SIGTERM', async () => {
  console.log('🛑 SIGTERM received, shutting down gracefully');

  // Close WebSocket connections
  wss.clients.forEach((ws) => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.close();
    }
  });

  // Close server
  server.close(() => {
    console.log('✅ Server shut down complete');
    process.exit(0);
  });
});

module.exports = { app, server };