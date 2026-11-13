import { Settings2 } from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { ROLE_LABELS } from "../utils/constants.js";
import { Avatar, PageIntro } from "../components/common/index.js";

export default function ProfilePage() {
  const { user } = useAuth();
  return (
    <>
      <PageIntro
        kicker="YOUR ACCOUNT"
        title="Profile"
        subtitle="The basics behind your learning space."
      />
      <section className="content-panel profile-panel">
        <div className="profile-hero">
          <Avatar name={user.name} size="xl" />
          <div>
            <span className="eyebrow">{ROLE_LABELS[user.role]}</span>
            <h2>{user.name}</h2>
            <p>{user.track || user.university || "EduFlow Institute"}</p>
          </div>
          <button
            className="button button-outline"
            onClick={() =>
              alert(
                "Profile editing will connect to the Go API when its contract is ready.",
              )
            }
          >
            <Settings2 size={15} /> Edit profile
          </button>
        </div>
        <div className="profile-details">
          <div>
            <span>Email address</span>
            <strong>{user.email}</strong>
          </div>
          <div>
            <span>Learning space</span>
            <strong>{user.university || "EduFlow Institute"}</strong>
          </div>
          <div>
            <span>Role</span>
            <strong>{ROLE_LABELS[user.role]}</strong>
          </div>
          <div>
            <span>Member since</span>
            <strong>September 2026</strong>
          </div>
        </div>
      </section>
    </>
  );
}
