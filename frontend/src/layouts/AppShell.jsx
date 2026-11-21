import { useEffect, useMemo, useRef, useState } from "react";
import {
  NavLink,
  Navigate,
  Outlet,
  useLocation,
  useNavigate,
} from "react-router-dom";
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
import { demoAssignments, demoNotifications } from "../data/assessments.js";
import { demoCourses } from "../data/courses.js";
import { demoSessions } from "../data/sessions.js";
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
  const [helpOpen, setHelpOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [searchFocused, setSearchFocused] = useState(false);
  const [notifications, setNotifications] = useState(demoNotifications);
  const searchInputRef = useRef(null);
  const location = useLocation();
  const navigate = useNavigate();

  useEffect(() => {
    function onKeyDown(e) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        searchInputRef.current?.focus();
        setSearchFocused(true);
      } else if (e.key === "Escape") {
        setSearchFocused(false);
        setNoticeOpen(false);
        setRoleOpen(false);
        setHelpOpen(false);
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  const searchResults = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return [];
    const courseMatches = demoCourses
      .filter((c) => `${c.title} ${c.category}`.toLowerCase().includes(q))
      .map((c) => ({
        id: c.id,
        title: c.title,
        subtitle: `${c.category} · ${c.duration}`,
        tag: "Course",
        to: `/courses/${c.id}`,
      }));
    const assignmentMatches = demoAssignments
      .filter((a) => `${a.title} ${a.description}`.toLowerCase().includes(q))
      .map((a) => ({
        id: a.id,
        title: a.title,
        subtitle: `Due ${a.dueDate}`,
        tag: "Work",
        to: "/assignments",
      }));
    const sessionMatches = demoSessions
      .filter((s) => `${s.title} ${s.location}`.toLowerCase().includes(q))
      .map((s) => ({
        id: s.id,
        title: s.title,
        subtitle: `${s.date} · ${s.startTime}`,
        tag: "Session",
        to: "/schedule",
      }));
    return [...courseMatches, ...assignmentMatches, ...sessionMatches].slice(
      0,
      6,
    );
  }, [searchQuery]);

  const pageTitle =
    navItems.find((item) => item.to === location.pathname)?.label ||
    (location.pathname.startsWith("/courses/")
      ? "Course overview"
      : "Your profile");
  const visibleItems = navItems.filter((item) =>
    item.roles.includes(user?.role || ROLES.STUDENT),
  );
  const groups = [...new Set(visibleItems.map((item) => item.group))];
  const unread = notifications.filter((item) => !item.read).length;

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
            type="button"
            className="icon-button sidebar-close"
            onClick={() => setMenuOpen(false)}
            aria-label="Close navigation"
          >
            <X size={18} />
          </button>
        </div>
        <div className="workspace-switch">
          <div className="workspace-avatar">EF</div>
          <div>
            <strong>Academic Term</strong>
            <small>FALL · 2026</small>
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
                    <Icon size={17} strokeWidth={1.85} />
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
          {helpOpen && (
            <div className="help-drawer">
              <div className="help-drawer-head">
                <strong>Quick shortcuts & support</strong>
                <button
                  type="button"
                  className="icon-button"
                  style={{ width: 24, height: 24 }}
                  onClick={() => setHelpOpen(false)}
                  aria-label="Close help"
                >
                  <X size={14} />
                </button>
              </div>
              <ul>
                <li>
                  <span>Global search</span>
                  <kbd>⌘ K</kbd>
                </li>
                <li>
                  <span>Close popovers</span>
                  <kbd>Esc</kbd>
                </li>
                <li>
                  <span>Advisor desk</span>
                  <strong>support@eduflow.dev</strong>
                </li>
              </ul>
            </div>
          )}
          <div className="help-card">
            <span className="help-icon">
              <CircleHelp size={16} />
            </span>
            <div>
              <strong>Need a hand?</strong>
              <p>Keyboard shortcuts and academic support.</p>
              <button
                type="button"
                onClick={() => setHelpOpen((prev) => !prev)}
              >
                {helpOpen ? "Hide quick guide" : "Open quick guide"}{" "}
                <ArrowRight size={13} />
              </button>
            </div>
          </div>
          <NavLink to="/profile" className="profile-link">
            <Avatar name={user.name} />
            <span className="profile-meta">
              <strong>{user.name}</strong>
              <small>{ROLE_LABELS[user.role]}</small>
            </span>
            <MoreHorizontal size={17} />
          </NavLink>
        </div>
      </aside>
      {menuOpen && (
        <button
          type="button"
          className="mobile-scrim"
          aria-label="Close menu"
          onClick={() => setMenuOpen(false)}
        />
      )}
      <main className="main-column">
        <header className="topbar">
          <button
            type="button"
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
            <div className="search-wrap">
              <label className="search-field">
                <Search size={15} />
                <input
                  ref={searchInputRef}
                  aria-label="Search courses, work, or sessions"
                  placeholder="Search courses, work…"
                  value={searchQuery}
                  onFocus={() => setSearchFocused(true)}
                  onChange={(e) => {
                    setSearchQuery(e.target.value);
                    setSearchFocused(true);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && searchResults.length > 0) {
                      navigate(searchResults[0].to);
                      setSearchFocused(false);
                      setSearchQuery("");
                    }
                  }}
                />
                <kbd>⌘K</kbd>
              </label>
              {searchFocused && searchQuery.trim().length > 0 && (
                <div className="search-results-popover">
                  {searchResults.length > 0 ? (
                    searchResults.map((res) => (
                      <button
                        type="button"
                        key={`${res.tag}-${res.id}`}
                        className="search-result-item"
                        onClick={() => {
                          navigate(res.to);
                          setSearchFocused(false);
                          setSearchQuery("");
                        }}
                      >
                        <span>
                          <strong>{res.title}</strong>
                          <small>{res.subtitle}</small>
                        </span>
                        <span className="search-result-tag">{res.tag}</span>
                      </button>
                    ))
                  ) : (
                    <div className="px-3 py-2.5 text-[12px] text-muted">
                      No matching courses, assignments, or sessions.
                    </div>
                  )}
                </div>
              )}
            </div>
            <div className="role-switch-wrap">
              <button
                type="button"
                className="icon-button notification-button"
                onClick={() => {
                  setNoticeOpen((prev) => !prev);
                  setRoleOpen(false);
                }}
                aria-label={`Notifications, ${unread} unread`}
              >
                <Bell size={18} />
                {unread > 0 && <span className="notification-dot" />}
              </button>
              {noticeOpen && (
                <NotificationPopover
                  notifications={notifications}
                  onMarkAllRead={() =>
                    setNotifications((prev) =>
                      prev.map((item) => ({ ...item, read: true })),
                    )
                  }
                  onClose={() => setNoticeOpen(false)}
                />
              )}
            </div>
            <div className="role-switch-wrap">
              <button
                type="button"
                className="top-user"
                onClick={() => {
                  setRoleOpen((prev) => !prev);
                  setNoticeOpen(false);
                }}
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
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -4 }}
              transition={{ duration: 0.16 }}
            >
              <Outlet />
            </motion.div>
          </AnimatePresence>
        </div>
      </main>
    </div>
  );
}
