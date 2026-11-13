import { NavLink, useParams } from "react-router-dom";
import {
  ArrowRight,
  CheckCircle2,
  CircleHelp,
  ShieldCheck,
} from "lucide-react";
import { demoCertificates } from "../data/assessments.js";
import { PRODUCT_NAME } from "../utils/constants.js";
import { BrandMark } from "../components/common/index.js";

export default function VerifyPage() {
  const { code } = useParams();
  const item = demoCertificates.find(
    (certificate) =>
      certificate.verificationCode.toLowerCase() === code.toLowerCase(),
  );
  return (
    <main className="verify-screen">
      <NavLink to="/" className="verify-brand">
        <BrandMark />
        <span className="brand-name">
          {PRODUCT_NAME.toLowerCase()}
          <span>.</span>
        </span>
      </NavLink>
      <section className="verify-card">
        <span className="verify-icon">
          {item ? <ShieldCheck size={25} /> : <CircleHelp size={25} />}
        </span>
        <span className="eyebrow">CREDENTIAL VERIFICATION</span>
        <h1>
          {item
            ? "This achievement is verified."
            : "We couldn’t find this credential."}
        </h1>
        {item ? (
          <>
            <p>This credential is listed as active in the EduFlow records.</p>
            <div className="verify-details">
              <span>
                Recipient<strong>{item.studentName}</strong>
              </span>
              <span>
                Achievement<strong>{item.courseName}</strong>
              </span>
              <span>
                Status
                <strong className="verified-status">
                  <CheckCircle2 size={15} /> Active
                </strong>
              </span>
            </div>
          </>
        ) : (
          <p>
            Check the verification code and try again. Only limited public
            information is shown here.
          </p>
        )}
        <span className="verify-code">{code}</span>
        <NavLink to="/" className="text-link">
          Return to EduFlow <ArrowRight size={14} />
        </NavLink>
      </section>
      <p className="verify-foot">
        EduFlow shares only the minimum information needed to verify a
        credential.
      </p>
    </main>
  );
}
