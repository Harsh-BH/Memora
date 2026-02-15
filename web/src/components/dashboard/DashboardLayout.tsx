"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import Sidebar from "./Sidebar";
import ChatInterface from "./ChatInterface";
import HippocampusView from "./HippocampusView";
import WorkspaceView from "./WorkspaceView";
import NeocortexView from "./NeocortexView";
import SleepConsole from "./SleepConsole";

export default function DashboardLayout() {
    const [activeView, setActiveView] = useState("chat");

    const renderContent = () => {
        switch (activeView) {
            case "chat":
                return <ChatInterface />;
            case "hippocampus":
                return <HippocampusView />;
            case "workspace":
                return <WorkspaceView />;
            case "neocortex":
                return <NeocortexView />;
            case "sleep":
                return <SleepConsole />;
            default:
                return <ChatInterface />;
        }
    };

    return (
        <div className="flex h-screen w-full bg-cream overflow-hidden font-sans text-charcoal">
            <Sidebar activeView={activeView} setActiveView={setActiveView} />

            <main className="flex-1 ml-64 h-full relative overflow-hidden bg-cream p-8">
                <AnimatePresence mode="wait">
                    <motion.div
                        key={activeView}
                        initial={{ opacity: 0, y: 10 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, y: -10 }}
                        transition={{ duration: 0.2 }}
                        className="h-full w-full overflow-y-auto pr-2"
                    >
                        {renderContent()}
                    </motion.div>
                </AnimatePresence>
            </main>
        </div>
    );
}
