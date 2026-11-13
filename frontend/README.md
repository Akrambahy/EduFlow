# EduFlow Frontend

Responsive React frontend for the EduFlow learning management system. The backend is a separate Go REST API owned by the project team.

## Stack

- Vite + React + JavaScript/JSX
- React Router
- Tailwind CSS 4 via the Vite plugin, with app-specific CSS tokens
- Motion for React (`motion/react`)
- Lucide icons

## Run locally

1. Install Node.js (22 or newer) and npm.
2. Install dependencies with `npm install`.
3. Copy `.env.example` to `.env` if you want to point at a backend.
4. Start the frontend with `npm run dev`.

The Vite server uses port `5173`. Without an API base URL, requests under `/api` are proxied to `http://localhost:8080`.

## Demo behavior and API connection

The current interface uses sample data in `src/data/` and the demo login provider in `src/hooks/useAuth.jsx`. This lets the role workspaces be explored before the Go API is connected. Demo accounts are listed on the sign-in page; clicking one fills its email. The demo password can be any non-empty value.

Configure `VITE_API_BASE_URL` when the backend contract and local address are known. The central client is in `src/services/api.js`. Confirm the authentication transport, role permissions, endpoint paths, grading policies, QR expiry, file rules, and certificate fields with the backend team before replacing demo behavior.

Frontend route checks and demo data are for presentation only. The Go API must remain authoritative for authentication, authorization, attendance, scores, standing, and certificate eligibility.

## Main routes

- `/login` — demo sign-in and role selection
- `/` — role-aware overview dashboard
- `/courses` and `/courses/:courseId` — course catalog and outline
- `/schedule` — upcoming and past sessions
- `/assignments` — assignments and quiz overview
- `/performance` — grade and attendance snapshot
- `/people` — learning community view for staff roles
- `/certificates` — issued credentials
- `/verify/:code` — minimal public verification page
- `/profile` — account information

Some actions remain demo placeholders until the Go API contract is available.

## Source structure

- `src/app/` — app-level routing and provider composition
- `src/pages/` — route page components
- `src/layouts/` — shared route layouts
- `src/components/common/` — shared app components, split by component
- `src/components/ui/` — reusable UI primitives
- `src/data/`, `src/hooks/`, `src/services/`, `src/utils/` — data, state, API, and utility modules
- `src/styles/` — Tailwind theme/base and styles grouped by app area; shared primitives use Tailwind utilities
- `src/assets/` — static assets
