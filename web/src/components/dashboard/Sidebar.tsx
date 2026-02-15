"use client";

import { motion } from "framer-motion";
import {
    LayoutDashboard,
    MessageSquare,
    BrainCircuit,
    Database,
    Moon,
    Settings,
    Cpu
} from "lucide-react";
import clsx from "clsx";

interface SidebarProps {
    activeView: string;
    setActiveView: (view: string) => void;
}

const menuItems = [
    {
        category: "Interface",
        items: [
            { id: "chat", label: "Wake Mode", icon: MessageSquare },
        ]
    },
    {
        category: "Core Architecture",
        items: [
            { id: "hippocampus", label: "Hippocampus", icon: Database }, // Ingest/Episodic
            { id: "workspace", label: "Workspace", icon: Cpu },           // Context/Knapsack
            { id: "neocortex", label: "Neocortex", icon: BrainCircuit },  // Knowledge Graph
        ]
    },
    {
        category: "System",
        items: [
            { id: "sleep", label: "Sleep Console", icon: Moon },
        ]
    }
];

export default function Sidebar({ activeView, setActiveView }: SidebarProps) {
    return (
        <motion.div
            initial={{ x: -20, opacity: 0 }}
            animate={{ x: 0, opacity: 1 }}
            className="w-64 h-screen bg-cream border-r border-charcoal/10 flex flex-col fixed left-0 top-0 z-50"
        >
            {/* Logo Area */}
            <div className="p-6 border-b border-charcoal/10">
                <div className="flex items-center gap-2">
                    <div className="w-8 h-8 rounded-lg bg-terracotta flex items-center justify-center">
                        <span className="text-white font-bold text-lg">M</span>
                    </div>
                    <div>
                        <h1 className="font-bold text-charcoal tracking-tight">Memora</h1>
                        <p className="text-[10px] text-terracotta font-mono uppercase tracking-wider">v1.2.0-beta</p>
                    </div>
                </div>
            </div>

            {/* Navigation */}
            <div className="flex-1 overflow-y-auto py-6 px-4 space-y-8">
                {menuItems.map((section) => (
                    <div key={section.category}>
                        <h3 className="text-xs font-semibold text-terracotta uppercase tracking-widest mb-3 px-2">
                            {section.category}
                        </h3>
                        <div className="space-y-1">
                            {section.items.map((item) => {
                                const Icon = item.icon;
                                const isActive = activeView === item.id;

                                return (
                                    <button
                                        key={item.id}
                                        onClick={() => setActiveView(item.id)}
                                        className={clsx(
                                            "w-full flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-medium transition-all duration-200 group relative",
                                            isActive
                                                ? "bg-white text-terracotta shadow-sm border border-terracotta/30"
                                                : "text-charcoal/80 hover:bg-terracotta/10 hover:text-terracotta"
                                        )}
                                    >
                                        <Icon
                                            size={18}
                                            className={clsx(
                                                "transition-colors",
                                                isActive ? "text-terracotta" : "text-terracotta/70 group-hover:text-terracotta"
                                            )}
                                        />
                                        {item.label}
                                        {isActive && (
                                            <motion.div
                                                layoutId="active-pill"
                                                className="absolute left-0 w-1 h-6 bg-terracotta rounded-r-full"
                                                initial={{ opacity: 0 }}
                                                animate={{ opacity: 1 }}
                                                transition={{ duration: 0.2 }}
                                            />
                                        )}
                                    </button>
                                );
                            })}
                        </div>
                    </div>
                ))}
            </div>

            {/* User Profile / Settings (Fixed Bottom) */}
            <div className="p-4 border-t border-charcoal/10">
                <button className="w-full flex items-center gap-3 px-3 py-2 rounded-xl text-sm font-medium text-charcoal hover:bg-terracotta/10 transition-colors">
                    <div className="w-8 h-8 rounded-full bg-terracotta/20 overflow-hidden">
                        <img src="https://api.dicebear.com/7.x/avataaars/svg?seed=Harsh" alt="User" />
                    </div>
                    <div className="text-left">
                        <p className="text-charcoal font-semibold text-xs">Harsh BH</p>
                        <p className="text-terracotta text-[10px]">Admin</p>
                    </div>
                    <Settings size={16} className="ml-auto text-terracotta" />
                </button>
            </div>
        </motion.div>
    );
}
