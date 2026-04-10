import { ArrowLeft } from "lucide-react";
import { Link } from "react-router-dom";

export function DashboardBackHomeLink() {
  return (
    <Link className="dashboard-page__home-link dashboard-page__home-link--floating" to="/">
      <ArrowLeft className="h-4 w-4" />
      返回首页
    </Link>
  );
}
