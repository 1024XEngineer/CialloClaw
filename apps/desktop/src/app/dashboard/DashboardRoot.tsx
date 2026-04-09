import { AnimatePresence, motion } from "motion/react";
import { HashRouter, Navigate, Route, Routes, useLocation } from "react-router-dom";
import { MemoryPage } from "@/features/dashboard/memory/MemoryPage";
import { NotesPage } from "@/features/dashboard/notes/NotesPage";
import { SafetyPage } from "@/features/dashboard/safety/SafetyPage";
import { TasksPage } from "@/features/dashboard/tasks/TasksPage";
import { DashboardHome } from "./DashboardHome";
import "./dashboard.css";

function DashboardRoutes() {
  const location = useLocation();

  return (
    <div className="dashboard-app">
      <AnimatePresence mode="wait">
        <motion.div
          key={location.pathname}
          animate={{ opacity: 1, y: 0 }}
          className="dashboard-route-layer"
          exit={{ opacity: 0, y: -12 }}
          initial={{ opacity: 0, y: 18 }}
          transition={{ duration: 0.38, ease: [0.22, 1, 0.36, 1] }}
        >
          <Routes location={location}>
            <Route element={<DashboardHome />} path="/" />
            <Route element={<TasksPage />} path="/tasks/*" />
            <Route element={<NotesPage />} path="/notes/*" />
            <Route element={<MemoryPage />} path="/memory/*" />
            <Route element={<SafetyPage />} path="/safety/*" />
            <Route element={<Navigate replace to="/" />} path="*" />
          </Routes>
        </motion.div>
      </AnimatePresence>
    </div>
  );
}

export function DashboardRoot() {
  return (
    <HashRouter>
      <DashboardRoutes />
    </HashRouter>
  );
}
