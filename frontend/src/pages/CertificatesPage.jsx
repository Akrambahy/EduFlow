import { NavLink } from "react-router-dom";
import { ArrowUpRight, Award } from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { demoCertificates } from "../data/assessments.js";
import { ROLES } from "../utils/constants.js";
import { prettyDate } from "../utils/formatters.js";
import { EmptyState, PageIntro } from "../components/common/index.js";

export default function CertificatesPage() {
  const { user } = useAuth();
  const items = demoCertificates.filter(
    (item) => user.role === ROLES.ADMIN || item.studentId === user.id,
  );
  return (
    <>
      <PageIntro
        kicker="RECOGNITION THAT TRAVELS WITH YOU"
        title="Certificates"
        subtitle="Celebrate milestones you’ve worked hard to reach."
      />
      <div className="certificate-list">
        {items.map((item) => (
          <article className="certificate-card" key={item.id}>
            <div className="certificate-cover">
              <div className="certificate-stamp">
                <Award size={25} />
              </div>
              <span className="certificate-brand">
                EDUFLOW / VERIFIED LEARNING
              </span>
              <span className="certificate-rule" />
              <h2>
                Certificate
                <br />
                <em>of achievement</em>
              </h2>
              <p>awarded to</p>
              <strong>{item.studentName}</strong>
              <span className="certificate-course">
                for completing {item.courseName}
              </span>
              <span className="certificate-code">{item.verificationCode}</span>
              <div className="certificate-flower">✳</div>
            </div>
            <div className="certificate-details">
              <span className="eyebrow">
                ISSUED {prettyDate(item.issueDate).toUpperCase()} ·{" "}
                {item.status.toUpperCase()}
              </span>
              <h3>{item.courseName}</h3>
              <p>
                Keep this milestone close. Your certificate can be verified
                using its unique code.
              </p>
              <div className="certificate-actions">
                <button
                  className="button button-dark"
                  onClick={() =>
                    alert(
                      "Download will be available when certificate files are connected.",
                    )
                  }
                >
                  <Award size={15} /> View certificate
                </button>
                <NavLink
                  className="button button-outline"
                  to={`/verify/${item.verificationCode}`}
                >
                  Verify credential <ArrowUpRight size={15} />
                </NavLink>
              </div>
            </div>
          </article>
        ))}
      </div>
      {items.length === 0 && (
        <EmptyState
          title="Your next milestone is ahead"
          text="When you meet a course’s completion requirements, your certificates will appear here."
        />
      )}
    </>
  );
}
