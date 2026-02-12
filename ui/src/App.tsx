import { BrowserRouter, Routes, Route } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "sonner";
import { AppLayout } from "@/components/layout/app-layout";
import { DashboardPage } from "@/components/dashboard/dashboard-page";
import { ProjectsPage } from "@/components/projects/projects-page";
import { ProjectDetail } from "@/components/projects/project-detail";
import { IssuesPage } from "@/components/issues/issues-page";
import { IssueDetail } from "@/components/issues/issue-detail";
import { SessionsPage } from "@/components/sessions/sessions-page";
import "./index.css";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
});

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route element={<AppLayout />}>
            <Route index element={<DashboardPage />} />
            <Route path="projects" element={<ProjectsPage />} />
            <Route path="projects/:id" element={<ProjectDetail />} />
            <Route path="issues" element={<IssuesPage />} />
            <Route path="issues/:id" element={<IssueDetail />} />
            <Route path="sessions" element={<SessionsPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
      <Toaster position="bottom-right" />
    </QueryClientProvider>
  );
}

export default App;
