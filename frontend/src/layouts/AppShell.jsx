import { useState } from "react";
import { NavLink, Navigate, Outlet, useLocation } from "react-router-dom";
import { AnimatePresence, motion } from "motion/react";
import {
  ArrowRight,
  Award,
  Bell,
  BookOpen,
  CalendarDays,
  ChevronDown,
  CircleHelp,
  ClipboardCheck,
  LayoutDashboard,
  Menu,
  MoreHorizontal,
  Search,
  TrendingUp,
  Users,
  X,
} from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { demoNotifications } from "../data/assessments.js";
import { PRODUCT_NAME, ROLE_LABELS, ROLES } from "../utils/constants.js";
import {
  Avatar,
  BrandMark,
  NotificationPopover,
  RoleMenu,
} from "../components/common/index.js";

const navItems = [
  {
    to: "/",
    label: "Overview",
    icon: LayoutDashboard,
    group: "Workspace",
    roles: Object.values(ROLES),
  },
  {
    to: "/courses",
    label: "Courses",
    icon: BookOpen,
    group: "Workspace",
    roles: Object.values(ROLES),
  },
  {
    to: "/schedule",
    label: "Schedule",
    icon: CalendarDays,
    group: "Workspace",
    roles: Object.values(ROLES),
  },
  {
    to: "/assignments",
    label: "Assignments",
    icon: ClipboardCheck,
    group: "Workspace",
    roles: Object.values(ROLES),
  },
  {
    to: "/performance",
    label: "Performance",
    icon: TrendingUp,
    group: "Insights",
    roles: Object.values(ROLES),
  },
  {
    to: "/people",
    label: "People",
    icon: Users,
    group: "Insights",
    roles: [ROLES.ADMIN, ROLES.INSTRUCTOR, ROLES.MENTOR, ROLES.TRACKER],
  },
  {
    to: "/certificates",
    label: "Certificates",
    icon: Award,
    group: "Insights",
    roles: [ROLES.STUDENT, ROLES.ADMIN],
  },
];

export default function AppShell() {
  const { user, logout, switchRole } = useAuth();
  const [menuOpen, setMenuOpen] = useState(false);
  const [noticeOpen, setNoticeOpen] = useState(false);
  const [roleOpen, setRoleOpen] = useState(false);
  const location = useLocation();
  const pageTitle =
    navItems.find((item) => item.to === location.pathname)?.label ||
    (location.pathname.startsWith("/courses/")
      ? "Course overview"
      : "Your profile");
  const visibleItems = navItems.filter((item) =>
    item.roles.includes(user?.role || ROLES.STUDENT),
  );
  const groups = [...new Set(visibleItems.map((item) => item.group))];
  const unread = demoNotifications.filter((item) => !item.read).length;

  if (!user) return <Navigate to="/login" replace />;
  return (
    <div className="app-frame">
      <aside className={`sidebar ${menuOpen ? "sidebar-open" : ""}`}>
        <div className="brand-row">
          <BrandMark />
          <span className="brand-name">
            {PRODUCT_NAME.toLowerCase()}
            <span>.</span>
          </span>
          <button
            className="icon-button sidebar-close"
            onClick={() => setMenuOpen(false)}
            aria-label="Close navigation"
          >
            <X size={18} />
          </button>
        </div>
        <div className="workspace-switch">
          <div className="workspace-avatar">E</div>
          <div>
            <strong>Learning space</strong>
            <small>Fall semester · 2026</small>
          </div>
          <ChevronDown size={15} />
        </div>
        <nav className="side-nav" aria-label="Main navigation">
          {groups.map((group) => (
            <div className="nav-group" key={group}>
              <span className="nav-caption">{group}</span>
              {visibleItems
                .filter((item) => item.group === group)
                .map(({ to, label, icon: Icon }) => (
                  <NavLink
                    key={to}
                    to={to}
                    end={to === "/"}
                    onClick={() => setMenuOpen(false)}
                    className={({ isActive }) =>
                      `nav-link ${isActive ? "nav-link-active" : ""}`
                    }
                  >
                    <Icon size={17} strokeWidth={1.8} />
                    <span>{label}</span>
                    {label === "Assignments" && (
                      <span className="nav-count">3</span>
                    )}
                  </NavLink>
                ))}
            </div>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="help-card">
            <span className="help-icon">
              <CircleHelp size={17} />
            </span>
            <div>
              <strong>Need a hand?</strong>
              <p>Your learning team is here to help.</p>
              <button
                onClick={() => alert("Help center is a demo placeholder.")}
              >
                Visit help center <ArrowRight size={13} />
              </button>
            </div>
          </div>
          <NavLink to="/profile" className="profile-link">
            <Avatar name={user.name} />
            <span className="profile-meta">
              <strong>{user.name}</strong>
              <small>{ROLE_LABELS[user.role]}</small>
            </span>
            <MoreHorizontal size={18} />
          </NavLink>
        </div>
      </aside>
      {menuOpen && (
        <button
          className="mobile-scrim"
          aria-label="Close menu"
          onClick={() => setMenuOpen(false)}
        />
      )}
      <main className="main-column">
        <header className="topbar">
          <button
            className="icon-button mobile-menu"
            onClick={() => setMenuOpen(true)}
            aria-label="Open navigation"
          >
            <Menu size={20} />
          </button>
          <div className="breadcrumb">
            <span>Workspace</span>
            <span className="breadcrumb-slash">/</span>
            <strong>{pageTitle}</strong>
          </div>
          <div className="topbar-actions">
            <label className="search-field">
              <Search size={16} />
              <input
                aria-label="Search"
                placeholder="Search anything…"
                onKeyDown={(e) =>
                  e.key === "Enter" && alert("Search is a demo placeholder.")
                }
              />
              <kbd>⌘ K</kbd>
            </label>
            <div className="role-switch-wrap">
              <button
                className="icon-button notification-button"
                onClick={() => setNoticeOpen(!noticeOpen)}
                aria-label={`Notifications, ${unread} unread`}
              >
                <Bell size={18} />
                {unread > 0 && <span className="notification-dot" />}
              </button>
              {noticeOpen && (
                <NotificationPopover onClose={() => setNoticeOpen(false)} />
              )}
            </div>
            <div className="role-switch-wrap">
              <button
                className="top-user"
                onClick={() => setRoleOpen(!roleOpen)}
              >
                <Avatar name={user.name} />
                <span>
                  <strong>{user.name}</strong>
                  <small>{ROLE_LABELS[user.role]}</small>
                </span>
                <ChevronDown size={14} />
              </button>
              {roleOpen && (
                <RoleMenu
                  onSelect={(role) => {
                    switchRole(role);
                    setRoleOpen(false);
                  }}
                  onLogout={logout}
                />
              )}
            </div>
          </div>
        </header>
        <div className="page-content">
          <AnimatePresence mode="wait">
            <motion.div
              key={location.pathname}
              initial={{ opacity: 0, y: 7 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -4 }}
              transition={{ duration: 0.18 }}
            >
              <Outlet />
            </motion.div>
          </AnimatePresence>
        </div>
      </main>
    </div>
  );
}
