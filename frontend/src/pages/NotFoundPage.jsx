import { NavLink } from "react-router-dom";
import { ArrowRight } from "lucide-react";
import { EmptyState } from "../components/common/index.js";

export default function NotFoundPage() {
  return (
    <EmptyState
      title="That page isn’t here"
      text="The link may have moved. Head back to your overview."
      action={
        <NavLink to="/" className="button button-dark">
          Go to overview <ArrowRight size={15} />
        </NavLink>
      }
    />
  );
}
