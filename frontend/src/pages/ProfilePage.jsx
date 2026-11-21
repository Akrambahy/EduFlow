import { useState } from "react";
import { Check, Settings2 } from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { ROLE_LABELS } from "../utils/constants.js";
import { Avatar, PageIntro } from "../components/common/index.js";

export default function ProfilePage() {
  const { user } = useAuth();
  const [editing, setEditing] = useState(false);
  const [displayName, setDisplayName] = useState(user.name);
  const [institution, setInstitution] = useState(
    user.track || user.university || "EduFlow Institute",
  );

  return (
    <>
      <PageIntro
        kicker="ACCOUNT SETTINGS"
        title="Profile"
        subtitle="Manage your academic profile and workspace preferences."
      />
      <section className="content-panel profile-panel">
        <div className="profile-hero">
          <Avatar name={displayName} size="xl" />
          <div>
            <span className="eyebrow">{ROLE_LABELS[user.role]}</span>
            <h2>{displayName}</h2>
            <p>{institution}</p>
          </div>
          <button
            type="button"
            className="button button-outline"
            onClick={() => setEditing((prev) => !prev)}
          >
            {editing ? (
              <>
                <Check size={15} /> Save changes
              </>
            ) : (
              <>
                <Settings2 size={15} /> Edit profile
              </>
            )}
          </button>
        </div>

        {editing ? (
          <div className="profile-details">
            <label className="field-group">
              Full name
              <input
                type="text"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
              />
            </label>
            <label className="field-group">
              Track / Institution
              <input
                type="text"
                value={institution}
                onChange={(e) => setInstitution(e.target.value)}
              />
            </label>
          </div>
        ) : (
          <div className="profile-details">
            <div>
              <span>Email address</span>
              <strong>{user.email}</strong>
            </div>
            <div>
              <span>Learning space</span>
              <strong>{institution}</strong>
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
        )}
      </section>
    </>
  );
}
