"use client";

import { motion } from "framer-motion";

function StatCard({
    label,
    value,
    sub,
    icon,
}: {
    label: string;
    value: string;
    sub: string;
    icon: string;
}) {
    return (
        <div className="flex-1 min-w-0">
            <p className="text-[10px] text-charcoal/60 mb-1 flex items-center gap-1">
                <span>{icon}</span> {label}
            </p>
            <p className="text-2xl font-bold text-charcoal">{value}</p>
            <p className="text-[9px] text-charcoal/60 mt-0.5">{sub}</p>
        </div>
    );
}

function MemoryBar({
    title,
    subtitle,
    statusColor,
    statusText,
    progress,
    percent,
}: {
    title: string;
    subtitle: string;
    statusColor: string;
    statusText: string;
    progress: number;
    percent: number;
}) {
    return (
        <div className="flex items-center gap-3 py-2">
            <div className="min-w-0 flex-shrink-0 w-[120px]">
                <p className="text-[11px] font-medium text-gray-700 truncate">
                    {title}
                </p>
                <p className="text-[9px] text-gray-400 truncate">{subtitle}</p>
            </div>
            <span
                className={`text-[8px] px-2 py-0.5 rounded-full font-medium flex-shrink-0 ${statusColor}`}
            >
                {statusText}
            </span>
            <div className="flex-1 bg-cream rounded-full h-2 overflow-hidden">
                <div
                    className="h-full bg-terracotta rounded-full transition-all duration-500"
                    style={{ width: `${progress}%` }}
                />
            </div>
            <span className="text-[10px] text-charcoal/60 font-medium flex-shrink-0 w-8 text-right">
                {percent}%
            </span>
        </div>
    );
}

function InsightCard({
    title,
    description,
    btn1,
    btn2,
}: {
    title: string;
    description: string;
    btn1: string;
    btn2: string;
}) {
    return (
        <div className="bg-cream rounded-lg p-3 mb-2">
            <p className="text-[10px] font-semibold text-charcoal">{title}</p>
            <p className="text-[9px] text-charcoal/60 mt-1 leading-relaxed">
                {description}
            </p>
            <div className="flex gap-2 mt-2">
                <button className="text-[8px] px-2 py-1 bg-white border border-terracotta/10 rounded text-charcoal hover:bg-cream">
                    {btn1}
                </button>
                <button className="text-[8px] px-2 py-1 bg-white border border-terracotta/10 rounded text-charcoal hover:bg-cream">
                    {btn2}
                </button>
            </div>
        </div>
    );
}

export default function DashboardMockup() {
    return (
        <motion.div
            initial={{ opacity: 0, y: 60, scale: 0.95 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            transition={{ duration: 0.9, delay: 0.8, ease: "easeOut" }}
            className="relative max-w-4xl mx-auto mt-16 mb-20"
        >
            {/* Botanical background */}
            <div className="absolute inset-0 -inset-x-32 -top-20 -bottom-20 z-0 flex items-center justify-center pointer-events-none">
                <img
                    src="/botanical.jpg"
                    alt="Botanical decoration"
                    className="w-full h-full object-contain opacity-80"
                />
            </div>

            {/* Dashboard Window */}
            <div className="relative z-10 bg-white rounded-2xl shadow-2xl shadow-charcoal/10 border border-terracotta/10 overflow-hidden mx-8">
                {/* Browser Chrome */}
                <div className="flex items-center gap-2 px-4 py-2.5 bg-cream border-b border-terracotta/10">
                    <div className="flex gap-1.5">
                        <div className="w-2.5 h-2.5 rounded-full bg-red-400" />
                        <div className="w-2.5 h-2.5 rounded-full bg-yellow-400" />
                        <div className="w-2.5 h-2.5 rounded-full bg-green-400" />
                    </div>
                    <div className="flex-1 flex items-center justify-center">
                        <div className="bg-white rounded-md px-4 py-1 text-[10px] text-charcoal/40 border border-terracotta/10 flex items-center gap-2 max-w-xs w-full">
                            <svg
                                width="10"
                                height="10"
                                viewBox="0 0 24 24"
                                fill="none"
                                stroke="currentColor"
                                strokeWidth="2"
                            >
                                <circle cx="11" cy="11" r="8" />
                                <path d="M21 21l-4.35-4.35" />
                            </svg>
                            <span>Query memories, episodes & knowledge...</span>
                        </div>
                    </div>
                    <div className="flex items-center gap-2">
                        <span className="bg-terracotta text-white text-[9px] px-3 py-1 rounded-md font-medium">
                            + Ingest Memory
                        </span>
                        <div className="w-6 h-6 rounded-full bg-beige" />
                    </div>
                </div>

                <div className="flex">
                    {/* Sidebar */}
                    <div className="w-[160px] bg-cream/50 border-r border-terracotta/10 p-3 flex-shrink-0">
                        <div className="flex items-center gap-2 mb-4">
                            <div className="w-5 h-5 rounded-full bg-gradient-to-br from-terracotta to-charcoal flex items-center justify-center">
                                <span className="text-[8px] text-white font-bold">M</span>
                            </div>
                            <span className="text-[11px] font-semibold text-charcoal">
                                Memora CMA
                            </span>
                        </div>

                        <div className="space-y-0.5">
                            <p className="text-[9px] text-charcoal/50 uppercase tracking-wider mb-2">
                                Memory System
                            </p>
                            {[
                                { name: "Dashboard", active: true, icon: "◻" },
                                { name: "Episodic Store", active: false, icon: "🧠" },
                                { name: "Knowledge Graph", active: false, icon: "🕸" },
                                { name: "Workspace", active: false, icon: "⚡" },
                                { name: "Consolidation", active: false, icon: "🌙" },
                                { name: "DIG Reranker", active: false, icon: "🎯" },
                                { name: "Metrics", active: false, icon: "📊" },
                                { name: "Settings", active: false, icon: "⚙" },
                            ].map((item) => (
                                <div
                                    key={item.name}
                                    className={`flex items-center gap-2 px-2 py-1.5 rounded-md text-[10px] ${item.active
                                        ? "bg-terracotta/10 text-terracotta font-medium"
                                        : "text-charcoal/60 hover:bg-cream"
                                        }`}
                                >
                                    <span className="text-[10px]">{item.icon}</span>
                                    {item.name}
                                </div>
                            ))}
                        </div>

                        <div className="mt-4">
                            <p className="text-[9px] text-charcoal/50 uppercase tracking-wider mb-2">
                                Recent Users
                            </p>
                            {["user_123", "user_456", "dev_session_01"].map((p) => (
                                <p key={p} className="text-[9px] text-charcoal/60 py-1 truncate">
                                    {p}
                                </p>
                            ))}
                        </div>

                        <div className="mt-auto pt-4 flex items-center gap-2 border-t border-terracotta/10 mt-4">
                            <div className="w-5 h-5 rounded-full bg-beige" />
                            <div>
                                <p className="text-[9px] font-medium text-charcoal">Admin</p>
                            </div>
                        </div>
                    </div>

                    {/* Main Content */}
                    <div className="flex-1 p-5 overflow-hidden">
                        {/* Welcome */}
                        <div className="mb-4">
                            <h2 className="text-lg font-bold text-charcoal">
                                Cognitive Memory Dashboard
                            </h2>
                            <p className="text-[10px] text-charcoal/60">
                                Real-time overview of the Continuum Memory Architecture — episodic encoding, semantic consolidation & workspace assembly.
                            </p>
                        </div>

                        {/* Stats Row */}
                        <div className="flex items-start gap-6 mb-5 pb-4 border-b border-terracotta/10">
                            <StatCard
                                icon="🧠"
                                label="Episodes Stored"
                                value="1,284"
                                sub="Hippocampus (Qdrant)"
                            />
                            <StatCard
                                icon="🕸"
                                label="Knowledge Triples"
                                value="3,741"
                                sub="Neocortex (Neo4j)"
                            />
                            <StatCard
                                icon="🌙"
                                label="Sleep Cycles"
                                value="47"
                                sub="DBSCAN consolidation runs"
                            />
                            <StatCard
                                icon="⚡"
                                label="Query Accuracy"
                                value="96%"
                                sub="DIG + Knapsack precision"
                            />
                            <div className="flex-shrink-0">
                                <button className="text-[9px] text-terracotta border border-terracotta/20 px-2 py-1 rounded-md hover:bg-terracotta/5">
                                    View Metrics →
                                </button>
                            </div>
                        </div>

                        <div className="flex gap-5">
                            {/* Left - Memory Stores */}
                            <div className="flex-1">
                                <div className="flex items-center gap-2 mb-3">
                                    <span className="text-sm">📋</span>
                                    <h3 className="text-[12px] font-bold text-charcoal">
                                        Memory Pipeline
                                    </h3>
                                </div>
                                <p className="text-[9px] text-charcoal/60 mb-3">
                                    Track the status of memory ingestion and consolidation
                                </p>

                                <MemoryBar
                                    title="Episodic Encoding"
                                    subtitle="Surprisal segmentation active"
                                    statusColor="bg-green-100 text-green-600"
                                    statusText="Active"
                                    progress={92}
                                    percent={92}
                                />
                                <MemoryBar
                                    title="Sleep Consolidation"
                                    subtitle="DBSCAN clustering · 12 pending episodes"
                                    statusColor="bg-orange-100 text-orange-600"
                                    statusText="Pending"
                                    progress={45}
                                    percent={45}
                                />
                                <MemoryBar
                                    title="Conflict Resolution"
                                    subtitle="Temporal decay active · 3 conflicts found"
                                    statusColor="bg-terracotta/10 text-terracotta"
                                    statusText="Resolving"
                                    progress={68}
                                    percent={68}
                                />
                            </div>

                            {/* Right - AI Insights */}
                            <div className="w-[220px] flex-shrink-0">
                                <div className="flex items-center gap-2 mb-3">
                                    <span className="text-sm">✨</span>
                                    <h3 className="text-[12px] font-bold text-charcoal">
                                        CMA Insights
                                    </h3>
                                </div>
                                <p className="text-[9px] text-charcoal/60 mb-3">
                                    Cognitive architecture recommendations
                                </p>

                                <InsightCard
                                    title="Consolidation recommended"
                                    description="User user_123 has 15 unconsolidated episodes. Trigger sleep cycle to synthesize knowledge triples."
                                    btn1="Consolidate"
                                    btn2="Dismiss"
                                />
                                <InsightCard
                                    title="Knowledge conflict detected"
                                    description="Entity 'workplace' has conflicting triples. Temporal decay suggests updating 'Google' → 'Meta' based on recency."
                                    btn1="Resolve"
                                    btn2="Review"
                                />
                            </div>
                        </div>

                        {/* Quick Actions */}
                        <div className="mt-4 pt-3 border-t border-terracotta/10">
                            <div className="flex items-center gap-2 mb-2">
                                <span className="text-sm">⚡</span>
                                <h3 className="text-[12px] font-bold text-charcoal">
                                    Quick Actions
                                </h3>
                            </div>
                            <p className="text-[9px] text-charcoal/60 mb-3">
                                Core memory operations
                            </p>
                            <div className="flex gap-2 flex-wrap">
                                {[
                                    { label: "Ingest Episode", icon: "📝" },
                                    { label: "Query Workspace", icon: "🔍" },
                                    { label: "Trigger Sleep Cycle", icon: "🌙" },
                                    { label: "View Knowledge Graph", icon: "🕸" },
                                ].map((action) => (
                                    <button
                                        key={action.label}
                                        className="text-[9px] text-charcoal/80 border border-terracotta/10 px-3 py-1.5 rounded-md hover:bg-cream flex items-center gap-1.5 transition-colors"
                                    >
                                        <span>{action.icon}</span>
                                        {action.label}
                                        <span className="text-charcoal/40 ml-1">⌘K</span>
                                    </button>
                                ))}
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </motion.div>
    );
}
