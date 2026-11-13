import { LogOut } from "lucide-react";
import { demoUsers } from "../../data/users.js";
import { ROLE_LABELS } from "../../utils/constants.js";
import Avatar from "./Avatar.jsx";

export default function RoleMenu({ onSelect, onLogout }) {
  return (
    <div className="popover role-popover">
      <span className="popover-title">SWITCH DEMO ROLE</span>
      {demoUsers.map((user) => (
        <button key={user.id} onClick={() => onSelect(user.role)}>
          <Avatar name={user.name} size="xs" />
          <span>{ROLE_LABELS[user.role]}</span>
          <small>{user.role === "student" ? "default" : "demo"}</small>
        </button>
      ))}
      <div className="popover-divider" />
      <button className="logout-action" onClick={onLogout}>
        <LogOut size={15} /> Sign out
      </button>
    </div>
  );
}
