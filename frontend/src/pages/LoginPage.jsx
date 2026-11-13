import { useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { ArrowRight, Sparkles } from "lucide-react";
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
          <span>LEARN WITH INTENTION</span>
        </div>
        <div className="login-art-copy">
          <span className="eyebrow light">A little progress, every day</span>
          <h1>
            Make room for
            <br />
            <em>what’s next.</em>
          </h1>
          <p>One thoughtful place for your courses, mentors, and momentum.</p>
        </div>
        <div className="orbit orbit-one" />
        <div className="orbit orbit-two" />
        <div className="login-art-foot">
          <span>EDUFLOW · LEARNING, IN MOTION</span>
          <span>01 / 05</span>
        </div>
      </section>
      <section className="login-panel">
        <div className="login-mobile-brand">
          <BrandMark />
        </div>
        <div className="login-form-wrap">
          <span className="eyebrow">WELCOME BACK</span>
          <h2>Good to see you.</h2>
          <p className="muted">Sign in to pick up where you left off.</p>
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
            <Sparkles size={15} />
            <span>Demo mode · use any role account below</span>
          </div>
          <div className="demo-account-list">
            {demoUsers.map((user) => (
              <button
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
          By continuing, you agree to the platform’s terms and privacy policy.
        </p>
      </section>
    </main>
  );
}
