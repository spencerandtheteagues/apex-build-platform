Build a complete, production-ready full-stack SaaS web app called “Apex FieldOps AI”.

The app is a contractor field operations platform for small contractors and field-service businesses. It must look and feel premium, investor-demo ready, and fully functional right after it is built.

Use this exact tech stack and style:
- React + TypeScript + Tailwind CSS + shadcn/ui components
- Dark navy/blue premium SaaS theme:
  - Background: #0F172A
  - Accents: glowing #22D3EE blue
  - Rounded panels, clean tables, generous spacing, subtle shadows, smooth hover states
- Fully responsive (mobile-first)
- All data stored in memory (no database, no external APIs, no real API keys needed)

Include rich, realistic demo data immediately on first load:
- 7 jobs with different statuses
- 4 crews
- 6 customers
- All numbers and calculations must be realistic

Navigation: Left sidebar with icons containing these exact pages:
1. Dashboard
2. Job Pipeline (Kanban)
3. New Job
4. Crew Management
5. Settings

Core requirements (all must work perfectly):

1. Dashboard
   - Five metric cards: Open Jobs, Pending Estimate Value, Accepted Job Value, Average Gross Margin, Jobs Needing Follow-up
   - Each card shows big number + small sparkline trend

2. Job Pipeline (Kanban)
   - Six columns: New Lead, Estimate Needed, Proposal Sent, Accepted, In Progress, Completed
   - Fully draggable cards (drag and drop between columns)
   - Dragging a job instantly updates its status and refreshes Dashboard metrics

3. New Job / Estimate Builder
   - Form fields: customer name, phone, email, address, job title, job type, urgency, project size (sq ft), labor hours, labor rate, materials cost, markup percentage, customer notes
   - All calculations update live as user types:
     - Labor cost, Material cost, Subtotal, Markup amount, Final customer price, Estimated profit, Gross margin %

4. Job Detail page
   - Shows full customer info, job info, estimate breakdown
   - Status dropdown, assigned crew dropdown, activity timeline, customer proposal preview
   - Large prominent button: “Launch Estimate Swarm”

5. Crew Management
   - List of crews with name, members, current jobs, availability

6. Settings page
   - Company name
   - Default labor rate
   - Default markup %
   - AI provider placeholders
   - Model routing table showing the three AI agents and their roles
   - “Reset Demo Data” button

Standout feature – Estimate Swarm (must look impressive):
- On Job Detail page, clicking “Launch Estimate Swarm” opens a full-screen modal
- Modal contains three side-by-side panels (glassmorphic cards with avatars):
  - Panel 1: Kimi K2.6 Orchestrator
  - Panel 2: GLM-5.1 Proposal Agent
  - Panel 3: DeepSeek V4 Risk Agent
- Each panel shows simulated streaming text (like real AI responses)
- After all three finish, display clean results section with:
  - Recommended final quote
  - Margin warning (if gross margin below 25%)
  - Risk flags
  - Customer-ready proposal text
  - Internal crew instructions
  - Next best action

Additional must-have polish:
- All forms have validation and success toast notifications
- Smooth animations and loading states where appropriate
- Every button and interaction must work instantly
- No console errors
- App must build and run perfectly on first try

Build the entire app so it is immediately ready for screen recording. Make the main user flow extremely smooth: Dashboard → New Job → Fill Estimate → Save → Job Detail → Launch Estimate Swarm.

Deliver the most polished, bug-free version possible.
