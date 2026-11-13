import { BrowserRouter, Route, Routes } from "react-router-dom";
import { AuthProvider } from "../hooks/useAuth.jsx";
import AppShell from "../layouts/AppShell.jsx";
import AssignmentsPage from "../pages/AssignmentsPage.jsx";
import CertificatesPage from "../pages/CertificatesPage.jsx";
import CourseDetailPage from "../pages/CourseDetailPage.jsx";
import CoursesPage from "../pages/CoursesPage.jsx";
import DashboardPage from "../pages/DashboardPage.jsx";
import LoginPage from "../pages/LoginPage.jsx";
import NotFoundPage from "../pages/NotFoundPage.jsx";
import PeoplePage from "../pages/PeoplePage.jsx";
import PerformancePage from "../pages/PerformancePage.jsx";
import ProfilePage from "../pages/ProfilePage.jsx";
import SchedulePage from "../pages/SchedulePage.jsx";
import VerifyPage from "../pages/VerifyPage.jsx";

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/verify/:code" element={<VerifyPage />} />
          <Route element={<AppShell />}>
            <Route index element={<DashboardPage />} />
            <Route path="courses" element={<CoursesPage />} />
            <Route path="courses/:courseId" element={<CourseDetailPage />} />
            <Route path="schedule" element={<SchedulePage />} />
            <Route path="assignments" element={<AssignmentsPage />} />
            <Route path="performance" element={<PerformancePage />} />
            <Route path="people" element={<PeoplePage />} />
            <Route path="certificates" element={<CertificatesPage />} />
            <Route path="profile" element={<ProfilePage />} />
            <Route path="*" element={<NotFoundPage />} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}
