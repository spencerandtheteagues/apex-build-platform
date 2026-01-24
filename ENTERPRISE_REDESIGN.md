# 🏗️ **APEX.BUILD ENTERPRISE PLATFORM REDESIGN**

## **LEAD ARCHITECT AGENT SPECIFICATIONS**

### **CORE DESIGN PHILOSOPHY**
Create a steampunk-industrial development platform that combines the sophistication of enterprise software with the aesthetic appeal of retro-futuristic design, directly competing with and surpassing Replit's interface quality.

### **COLOR PALETTE & AESTHETIC**
```css
/* Primary Colors */
--bg-primary: #0a0a0a;           /* Deep black backgrounds */
--bg-secondary: #1a1a1a;         /* Slightly lighter panels */
--bg-tertiary: #2a2a2a;          /* Elevated surfaces */

/* Neon Accents */
--neon-cyan: #00f5ff;            /* Primary accent - buttons, borders */
--neon-green: #39ff14;           /* Success states, active indicators */
--neon-pink: #ff0080;            /* Action buttons, highlights */
--neon-orange: #ff6600;          /* Warning states, secondary actions */
--neon-purple: #8a2be2;          /* Special features, premium indicators */

/* Text Colors */
--text-primary: #ffffff;         /* Main text */
--text-secondary: #cccccc;       /* Subdued text */
--text-muted: #888888;           /* Placeholder text */

/* Steampunk Elements */
--brass: #cd7f32;                /* Metallic accents */
--copper: #b87333;               /* Pipe elements */
--steam: rgba(255,255,255,0.1);  /* Steam/glow effects */
```

### **3D BUTTON DESIGN SYSTEM**
```css
/* Glowing 3D Button Base */
.btn-3d {
  position: relative;
  padding: 12px 24px;
  background: linear-gradient(145deg, #2a2a2a, #1a1a1a);
  border: 2px solid var(--neon-cyan);
  border-radius: 8px;
  color: var(--text-primary);
  text-transform: uppercase;
  font-weight: bold;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);

  /* 3D Depth */
  box-shadow:
    0 4px 8px rgba(0, 0, 0, 0.3),
    0 0 20px rgba(0, 245, 255, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);

  /* Steampunk texture */
  background-image:
    radial-gradient(circle at 20% 20%, rgba(205, 127, 50, 0.1) 0%, transparent 50%),
    linear-gradient(145deg, transparent 70%, rgba(0, 245, 255, 0.05) 100%);
}

.btn-3d:hover {
  transform: translateY(-2px) scale(1.02);
  box-shadow:
    0 8px 16px rgba(0, 0, 0, 0.4),
    0 0 30px rgba(0, 245, 255, 0.4),
    inset 0 1px 0 rgba(255, 255, 255, 0.2);
  border-color: var(--neon-green);
}

.btn-3d:active {
  transform: translateY(0) scale(0.98);
  box-shadow:
    0 2px 4px rgba(0, 0, 0, 0.3),
    0 0 15px rgba(0, 245, 255, 0.3),
    inset 0 2px 4px rgba(0, 0, 0, 0.2);
}
```

### **LAYOUT ARCHITECTURE**

#### **1. MAIN DASHBOARD STRUCTURE**
```
┌─────────────────────────────────────────────────┐
│ HEADER: Logo + Navigation + User Profile       │
├─────────────────────────────────────────────────┤
│ HERO SECTION: Prominent Prompt Input Field     │
│   ┌─────────────────────────────────────────┐   │
│   │ "Describe the app you want to build"    │   │
│   │ [Large Text Area]                       │   │
│   │ [AI Model Selector] [Generate App Btn]  │   │
│   └─────────────────────────────────────────┘   │
├─────────────────────────────────────────────────┤
│ MAIN CONTENT GRID:                             │
│ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ │
│ │   Recent    │ │   Projects  │ │ AI Models   │ │
│ │   Apps      │ │  Explorer   │ │   Status    │ │
│ └─────────────┘ └─────────────┘ └─────────────┘ │
├─────────────────────────────────────────────────┤
│ FOOTER: Status Bar + System Info               │
└─────────────────────────────────────────────────┘
```

#### **2. CODE EDITOR INTERFACE**
```
┌─────────────────────────────────────────────────┐
│ PROJECT HEADER: Name + Status + Actions        │
├───────────┬─────────────────────────┬───────────┤
│   FILE    │      EDITOR AREA        │    AI     │
│ EXPLORER  │                         │ ASSISTANT │
│           │  ┌─────────────────────┐ │           │
│  📁 src/  │  │                     │ │ 🤖 Ready │
│  📄 main  │  │    CODE EDITOR      │ │   to      │
│  📄 css   │  │   (Monaco/VS Code)  │ │  help!    │
│  📄 html  │  │                     │ │           │
│           │  └─────────────────────┘ │ [Chat UI] │
├───────────┼─────────────────────────┤           │
│ TERMINAL  │     PREVIEW PANE        │           │
│ $ npm run │  ┌─────────────────────┐ │           │
│ $ build   │  │   LIVE APP PREVIEW  │ │           │
│ $ deploy  │  │                     │ │           │
└───────────┴─────────────────────────┴───────────┘
```

### **CRITICAL DESIGN QUESTIONS FOR APPROVAL:**

**Question 1: PROMPT INPUT PLACEMENT**
Where should the primary prompt input be positioned?
A) Hero section at top of dashboard (most prominent)
B) Floating overlay that appears on button click
C) Dedicated page accessible from main navigation
D) Sidebar that slides in when needed

**Question 2: AI MODEL SELECTION**
How should users choose between Claude/GPT/Gemini?
A) Dropdown selector next to prompt input
B) Tab interface above prompt area
C) Advanced settings modal
D) Automatic selection based on prompt analysis

**Question 3: APP PREVIEW INTEGRATION**
Where should generated apps be previewed?
A) Split pane within editor interface
B) Full-screen overlay with close button
C) New tab/window (external)
D) Embedded iframe in dedicated preview section

**Question 4: STEAMPUNK INTENSITY**
How heavily should steampunk elements be applied?
A) Subtle accents (gears, brass borders, industrial typography)
B) Moderate theming (pipe borders, steam effects, copper highlights)
C) Heavy theming (mechanical animations, complex industrial UI)
D) Full immersion (animated gears, steam particles, complex 3D)

**AWAITING YOUR DECISIONS TO PROCEED WITH DETAILED IMPLEMENTATION.**