"use client";

import { useState, useEffect } from "react";
import { BrainCircuit, RefreshCw } from "lucide-react";
import { motion } from "framer-motion";
import { cmaApi } from "@/lib/cma-api";

export default function NeocortexView() {
    const [stats, setStats] = useState({ nodes: 0, edges: 0 });
    const [loading, setLoading] = useState(true);
    const userId = "harsh1";

    const fetchStats = async () => {
        try {
            const data = await cmaApi.getNeocortex(userId);
            setStats(data);
        } catch (error) {
            console.error(error);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchStats();
        const interval = setInterval(fetchStats, 10000); // Poll every 10s
        return () => clearInterval(interval);
    }, []);

    const density = stats.nodes > 0 ? (stats.edges / stats.nodes).toFixed(2) : "0.00";

    const graphStats = [
        { label: "Entities", value: stats.nodes.toLocaleString() },
        { label: "Relations", value: stats.edges.toLocaleString() },
        { label: "Density", value: density },
    ];

    return (
        <div className="space-y-6 h-full flex flex-col">
            <header className="shrink-0">
                <h2 className="text-2xl font-bold text-terracotta tracking-tight flex items-center gap-2">
                    <BrainCircuit className="text-beige" size={24} />
                    Neocortex
                    {loading && <RefreshCw size={14} className="animate-spin ml-2 opacity-50" />}
                </h2>
                <p className="text-charcoal/60 text-sm">Semantic knowledge graph and consolidated facts.</p>
            </header>

            {/* Stats */}
            <div className="grid grid-cols-3 gap-4 shrink-0">
                {graphStats.map((stat) => (
                    <div key={stat.label} className="bg-white p-4 rounded-2xl border border-terracotta/20 shadow-sm text-center">
                        <p className="text-xs font-semibold text-charcoal/60 uppercase tracking-wider">{stat.label}</p>
                        <p className="text-2xl font-bold text-terracotta mt-1">{stat.value}</p>
                    </div>
                ))}
            </div>

            {/* Graph Visualization (Simulated) */}
            <div className="flex-1 bg-charcoal rounded-2xl border border-white/10 shadow-inner relative overflow-hidden group">
                {/* Grid Background */}
                <div className="absolute inset-0 opacity-20"
                    style={{ backgroundImage: 'radial-gradient(circle, #B17457 1px, transparent 1px)', backgroundSize: '30px 30px' }}
                />

                {/* Simulated Nodes */}
                <div className="absolute inset-0 flex items-center justify-center">
                    {/* Central Node */}
                    <motion.div
                        animate={{ scale: [1, 1.1, 1] }}
                        transition={{ duration: 4, repeat: Infinity }}
                        className="absolute w-24 h-24 bg-terracotta/20 rounded-full border border-terracotta flex items-center justify-center z-10 backdrop-blur-sm"
                    >
                        <span className="text-cream text-xs font-bold">Concept</span>
                    </motion.div>

                    {/* Orbiting Nodes */}
                    {[...Array(6)].map((_, i) => (
                        <motion.div
                            key={i}
                            className="absolute w-12 h-12 bg-beige/10 rounded-full border border-beige/50 flex items-center justify-center"
                            animate={{ rotate: 360 }}
                            style={{ translateX: 100 + i * 20 }}
                            transition={{ duration: 10 + i * 2, repeat: Infinity, ease: "linear" }}
                        >
                            <div className="w-2 h-2 bg-beige rounded-full" />
                        </motion.div>
                    ))}

                    {/* Connecting Lines (Static SVG overlay for effect) */}
                    <svg className="absolute inset-0 w-full h-full pointer-events-none opacity-30">
                        <line x1="50%" y1="50%" x2="60%" y2="30%" stroke="#B17457" strokeWidth="1" />
                        <line x1="50%" y1="50%" x2="30%" y2="60%" stroke="#B17457" strokeWidth="1" />
                        <line x1="50%" y1="50%" x2="70%" y2="70%" stroke="#B17457" strokeWidth="1" />
                    </svg>
                </div>

                <div className="absolute bottom-4 right-4 bg-charcoal/80 px-3 py-1.5 rounded-lg text-xs text-cream border border-white/20 backdrop-blur">
                    Live View: Active
                </div>
            </div>
        </div>
    );
}
