import { motion } from "motion/react";
import { MoreHorizontal, Plus } from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { demoTracks } from "../data/tracks.js";
import { demoUsers } from "../data/users.js";
import { ROLE_LABELS, ROLES } from "../utils/constants.js";
import { Avatar, PageIntro } from "../components/common/index.js";

export default function PeoplePage() {
  const { user } = useAuth();
  const people = demoUsers.filter(
    (person) =>
      person.role === ROLES.STUDENT ||
      person.role === ROLES.INSTRUCTOR ||
      person.role === ROLES.MENTOR ||
      person.role === ROLES.TRACKER,
  );
  return (
    <>
      <PageIntro
        kicker="YOUR LEARNING COMMUNITY"
        title={user.role === ROLES.ADMIN ? "People" : "Your people"}
        subtitle="The learners and educators connected to your space."
        action={
          user.role === ROLES.ADMIN ? (
            <button
              className="button button-dark"
              onClick={() =>
                alert(
                  "Invitations will connect to the user API when available.",
                )
              }
            >
              <Plus size={16} /> Invite someone
            </button>
          ) : null
        }
      />
      <div className="people-summary">
        <div>
          <span className="eyebrow">ASSIGNED SCOPE</span>
          <strong>
            {user.role === ROLES.ADMIN ? "128" : "42"}
            <small> people</small>
          </strong>
        </div>
        <div>
          <span className="eyebrow">ACTIVE TRACKS</span>
          <strong>
            {demoTracks.length}
            <small> tracks</small>
          </strong>
        </div>
        <div>
          <span className="eyebrow">NEEDS A CHECK-IN</span>
          <strong>
            06<small> learners</small>
          </strong>
        </div>
      </div>
      <div className="people-grid">
        {people.map((person, index) => (
          <motion.article
            key={person.id}
            className="person-card"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: index * 0.04 }}
          >
            <div className="person-card-top">
              <Avatar name={person.name} size="lg" />
              <button
                className="icon-button"
                aria-label={`More options for ${person.name}`}
                onClick={() => alert("People actions are a demo placeholder.")}
              >
                <MoreHorizontal size={18} />
              </button>
            </div>
            <h3>{person.name}</h3>
            <p>{ROLE_LABELS[person.role]}</p>
            <div className="person-track">
              <span>
                {person.role === ROLES.STUDENT
                  ? "Full-Stack Development"
                  : "Learning team"}
              </span>
              <span className="person-status">
                <i /> Active
              </span>
            </div>
          </motion.article>
        ))}
      </div>
    </>
  );
}
