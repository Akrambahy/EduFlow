import { useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { ArrowRight, ShieldCheck } from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { demoUsers } from "../data/users.js";
import { ROLE_LABELS } from "../utils/constants.js";
import { BrandMark } from "../components/common/index.js";

export default function LoginPage() {
  const { login, isAuthenticated } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState("student@eduflow.dev");
  const [password, setPassword] = useState("demo");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  if (isAuthenticated) return <Navigate to="/" replace />;

  async function handleSubmit(event) {
    event.preventDefault();
    setError("");
    setLoading(true);
    try {
      await login(email, password);
      navigate("/");
    } catch (err) {
      setError(
        err.message || "Sign in failed. Check your details and try again.",
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="login-screen">
      <section className="login-art" aria-label="EduFlow learning platform">
        <div className="login-art-top">
          <BrandMark variant="light" />
          <span>EDUFLOW ACADEMIC PORTAL</span>
        </div>
        <div className="login-art-copy">
          <span className="eyebrow light">STRUCTURED COHORT LEARNING</span>
          <h1>
            Focused coursework,
            <br />
            <em>clear progress.</em>
          </h1>
          <p>
            Manage courses, submit engineering deliverables, and track verified
            academic milestones in one workspace.
          </p>
          <div className="login-metrics-card">
            <div>
              <strong>04</strong>
              <span>Active courses</span>
            </div>
            <div>
              <strong>90%</strong>
              <span>Cohort attendance</span>
            </div>
            <div>
              <strong>24/7</strong>
              <span>Credential verify</span>
            </div>
          </div>
        </div>
        <div className="login-art-foot">
          <span>EDUFLOW · FALL SEMESTER 2026</span>
          <span>v2.0 ACADEMIC</span>
        </div>
      </section>
      <section className="login-panel">
        <div className="login-mobile-brand">
          <BrandMark />
        </div>
        <div className="login-form-wrap">
          <span className="eyebrow">SIGN IN</span>
          <h2>Access your workspace.</h2>
          <p className="muted">Enter your credentials or select a demo role.</p>
          <form onSubmit={handleSubmit} className="login-form">
            <label>
              Email address
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                autoComplete="username"
                required
              />
            </label>
            <label>
              Password
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                required
              />
            </label>
            {error && (
              <p className="form-error" role="alert">
                {error}
              </p>
            )}
            <button
              className="button button-dark button-wide"
              disabled={loading}
            >
              {loading ? "Signing in…" : "Sign in"}
              <ArrowRight size={17} />
            </button>
          </form>
          <div className="demo-hint">
            <ShieldCheck size={15} />
            <span>Quick role switch · select an account</span>
          </div>
          <div className="demo-account-list">
            {demoUsers.map((user) => (
              <button
                type="button"
                key={user.id}
                onClick={() => {
                  setEmail(user.email);
                  setPassword("demo");
                }}
              >
                <span>{ROLE_LABELS[user.role]}</span>
                <small>{user.email}</small>
              </button>
            ))}
          </div>
        </div>
        <p className="login-legal">
          By continuing, you agree to the platform’s academic integrity and
          privacy policies.
        </p>
      </section>
    </main>
  );
}
