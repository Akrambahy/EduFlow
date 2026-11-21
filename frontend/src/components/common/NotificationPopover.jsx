import { Bell, Check, X } from "lucide-react";
import { useNavigate } from "react-router-dom";

export default function NotificationPopover({
  notifications,
  onMarkAllRead,
  onClose,
}) {
  const navigate = useNavigate();
  const unreadCount = notifications.filter((n) => !n.read).length;

  return (
    <div className="popover notification-popover" role="dialog" aria-label="Notifications">
      <div className="notification-popover-head">
        <span>
          <strong>Notifications</strong>
          <small>
            {unreadCount === 0
              ? "All caught up"
              : `${unreadCount} unread update${unreadCount > 1 ? "s" : ""}`}
          </small>
        </span>
        <button
          type="button"
          className="icon-button"
          aria-label="Close notifications"
          onClick={onClose}
        >
          <X size={16} />
        </button>
      </div>
      {notifications.slice(0, 4).map((item) => (
        <div
          className={`notification-item ${!item.read ? "notification-unread" : ""}`}
          key={item.id}
          onClick={() => {
            onClose();
            if (item.link) {
              navigate(item.link.includes("/assignments") ? "/assignments" : item.link);
            }
          }}
          style={{ cursor: "pointer" }}
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
      {unreadCount > 0 && (
        <button type="button" className="mark-read" onClick={onMarkAllRead}>
          Mark all as read <Check size={14} />
        </button>
      )}
    </div>
  );
}
