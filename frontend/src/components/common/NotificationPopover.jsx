import { useState } from "react";
import { Bell, Check, X } from "lucide-react";
import { demoNotifications } from "../../data/assessments.js";

export default function NotificationPopover({ onClose }) {
  const [read, setRead] = useState(false);

  return (
    <div className="popover notification-popover">
      <div className="notification-popover-head">
        <span>
          <strong>Notifications</strong>
          <small>{read ? "All caught up" : "2 unread updates"}</small>
        </span>
        <button
          className="icon-button"
          aria-label="Close notifications"
          onClick={onClose}
        >
          <X size={16} />
        </button>
      </div>
      {demoNotifications.slice(0, 3).map((item) => (
        <div
          className={`notification-item ${!item.read && !read ? "notification-unread" : ""}`}
          key={item.id}
        >
          <span className="notification-icon">
            <Bell size={14} />
          </span>
          <span>
            <strong>{item.title}</strong>
            <small>{item.message}</small>
          </span>
        </div>
      ))}
      <button className="mark-read" onClick={() => setRead(true)}>
        Mark all as read <Check size={14} />
      </button>
    </div>
  );
}
