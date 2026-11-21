import { useState } from "react";
import { NavLink } from "react-router-dom";
import { ArrowUpRight, Award, Printer, X } from "lucide-react";
import { useAuth } from "../hooks/useAuth.jsx";
import { demoCertificates } from "../data/assessments.js";
import { ROLES } from "../utils/constants.js";
import { prettyDate } from "../utils/formatters.js";
import { EmptyState, PageIntro } from "../components/common/index.js";

export default function CertificatesPage() {
  const { user } = useAuth();
  const [previewCert, setPreviewCert] = useState(null);
  const items = demoCertificates.filter(
    (item) => user.role === ROLES.ADMIN || item.studentId === user.id,
  );
  return (
    <>
      <PageIntro
        kicker="VERIFIED CREDENTIALS"
        title="Certificates"
        subtitle="Official completion records with public verification codes."
      />
      <div className="certificate-list">
        {items.map((item) => (
          <article className="certificate-card" key={item.id}>
            <div className="certificate-cover">
              <div className="certificate-stamp">
                <Award size={22} />
              </div>
              <span className="certificate-brand">
                EDUFLOW · VERIFIED CREDENTIAL
              </span>
              <span className="certificate-rule" />
              <h2>
                Certificate
                <br />
                <em>of Achievement</em>
              </h2>
              <p>awarded to</p>
              <strong>{item.studentName}</strong>
              <span className="certificate-course">
                for completing {item.courseName}
              </span>
              <span className="certificate-code">{item.verificationCode}</span>
            </div>
            <div className="certificate-details">
              <span className="eyebrow">
                ISSUED {prettyDate(item.issueDate).toUpperCase()} ·{" "}
                {item.status.toUpperCase()}
              </span>
              <h3>{item.courseName}</h3>
              <p>
                Final grade: <strong>{item.grade}%</strong>. Share your
                verification link or print an official copy for your portfolio.
              </p>
              <div className="certificate-actions">
                <button
                  type="button"
                  className="button button-dark"
                  onClick={() => setPreviewCert(item)}
                >
                  <Award size={15} /> Preview & print
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
          title="No certificates issued yet"
          text="Complete all required modules and assessments in a course to earn your verified certificate."
        />
      )}

      {previewCert && (
        <div className="modal-backdrop" role="dialog" aria-modal="true">
          <div className="modal-card">
            <div className="modal-header">
              <h3>Official Credential Preview</h3>
              <button
                type="button"
                className="icon-button"
                onClick={() => setPreviewCert(null)}
                aria-label="Close preview"
              >
                <X size={16} />
              </button>
            </div>
            <div className="modal-body">
              <div className="rounded-lg border border-slate-200 bg-slate-50 p-5">
                <span className="eyebrow">EDUFLOW ACADEMIC RECORD</span>
                <h4 className="mt-2 font-display text-[18px] font-bold text-ink">
                  {previewCert.courseName}
                </h4>
                <p className="mt-1 text-[13px] text-slate-700">
                  Awarded to <strong>{previewCert.studentName}</strong> on{" "}
                  {prettyDate(previewCert.issueDate)} with a final grade of{" "}
                  <strong>{previewCert.grade}%</strong>.
                </p>
                <div className="mt-3 font-mono text-[12px] font-semibold text-blue-700">
                  Verification ID: {previewCert.verificationCode}
                </div>
              </div>
            </div>
            <div className="modal-footer">
              <button
                type="button"
                className="button button-outline"
                onClick={() => setPreviewCert(null)}
              >
                Close
              </button>
              <button
                type="button"
                className="button button-dark"
                onClick={() => window.print()}
              >
                <Printer size={14} /> Print / Save PDF
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
