"use client";

import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from "recharts";
import { Activity, Clock, Zap } from "lucide-react";

const SURPRISAL_DATA = [
    { time: "10:00", value: 0.2 },
    { time: "10:01", value: 0.3 },
    { time: "10:02", value: 0.8 }, // Spike
    { time: "10:03", value: 0.4 },
    { time: "10:04", value: 0.2 },
    { time: "10:05", value: 0.9 }, // Spike
    { time: "10:06", value: 0.5 },
    { time: "10:07", value: 0.3 },
];

const RECENT_EPISODES = [
    { id: "ep_10293", content: "Explained difference between Hippocampus and Neocortex.", surprisal: 0.85, time: "2m ago" },
    { id: "ep_10292", content: "User asked about system architecture.", surprisal: 0.45, time: "5m ago" },
    { id: "ep_10291", content: "Session start.", surprisal: 0.12, time: "8m ago" },
];

export default function HippocampusView() {
    return (
        <div className="space-y-6">
            <header>
                <h2 className="text-2xl font-bold text-terracotta tracking-tight flex items-center gap-2">
                    <Activity className="text-beige" size={24} />
                    Hippocampus
                </h2>
                <p className="text-charcoal/60 text-sm">Episodic memory ingestion and surprisal-based segmentation.</p>
            </header>

            {/* Metrics Grid */}
            <div className="grid grid-cols-3 gap-4">
                <div className="bg-white p-4 rounded-2xl border border-terracotta/20 shadow-sm">
                    <div className="flex items-center gap-2 mb-2">
                        <Zap size={16} className="text-beige" />
                        <h3 className="text-xs font-semibold text-charcoal/60 uppercase">Avg Surprisal</h3>
                    </div>
                    <p className="text-2xl font-bold text-terracotta">0.42</p>
                </div>
                <div className="bg-white p-4 rounded-2xl border border-terracotta/20 shadow-sm">
                    <div className="flex items-center gap-2 mb-2">
                        <Clock size={16} className="text-terracotta" />
                        <h3 className="text-xs font-semibold text-charcoal/60 uppercase">Ingest Latency</h3>
                    </div>
                    <p className="text-2xl font-bold text-terracotta">12ms</p>
                </div>
                <div className="bg-white p-4 rounded-2xl border border-terracotta/20 shadow-sm">
                    <div className="flex items-center gap-2 mb-2">
                        <Activity size={16} className="text-charcoal/60" />
                        <h3 className="text-xs font-semibold text-charcoal/60 uppercase">Episodes Today</h3>
                    </div>
                    <p className="text-2xl font-bold text-terracotta">143</p>
                </div>
            </div>

            {/* Chart */}
            <div className="bg-white p-6 rounded-2xl border border-terracotta/20 shadow-sm">
                <div className="flex justify-between items-center mb-6">
                    <h3 className="font-semibold text-terracotta">Surprisal Metric Stream</h3>
                    <div className="px-2 py-1 bg-cream rounded text-[10px] font-mono text-charcoal/60">
                        S(x_t) = -log P(x_t | x_&lt;t)
                    </div>
                </div>
                <div className="h-64 w-full">
                    <ResponsiveContainer width="100%" height="100%">
                        <AreaChart data={SURPRISAL_DATA}>
                            <defs>
                                <linearGradient id="colorSurprisal" x1="0" y1="0" x2="0" y2="1">
                                    <stop offset="5%" stopColor="#B17457" stopOpacity={0.4} />
                                    <stop offset="95%" stopColor="#B17457" stopOpacity={0} />
                                </linearGradient>
                            </defs>
                            <XAxis dataKey="time" stroke="#4A4947" fontSize={12} tickLine={false} axisLine={false} />
                            <YAxis stroke="#4A4947" fontSize={12} tickLine={false} axisLine={false} />
                            <Tooltip
                                contentStyle={{ backgroundColor: '#FAF7F0', borderRadius: '8px', border: '1px solid #B17457' }}
                                itemStyle={{ color: '#B17457' }}
                            />
                            <Area
                                type="monotone"
                                dataKey="value"
                                stroke="#B17457"
                                strokeWidth={2}
                                fillOpacity={1}
                                fill="url(#colorSurprisal)"
                            />
                        </AreaChart>
                    </ResponsiveContainer>
                </div>
            </div>

            {/* Episodes List */}
            <div className="bg-white rounded-2xl border border-terracotta/20 shadow-sm overflow-hidden">
                <div className="p-4 border-b border-terracotta/20 flex justify-between items-center">
                    <h3 className="font-semibold text-terracotta">Recent Episodes</h3>
                    <button className="text-xs text-beige font-medium hover:text-terracotta">View All</button>
                </div>
                <div className="divide-y divide-terracotta/10">
                    {RECENT_EPISODES.map((ep) => (
                        <div key={ep.id} className="p-4 flex items-center justify-between hover:bg-cream transition-colors">
                            <div className="space-y-1">
                                <p className="text-sm text-terracotta font-medium">{ep.content}</p>
                                <div className="flex items-center gap-2">
                                    <span className="text-[10px] bg-cream text-charcoal/60 px-1.5 py-0.5 rounded font-mono">{ep.id}</span>
                                    <span className="text-[10px] text-charcoal/60">{ep.time}</span>
                                </div>
                            </div>
                            <div className="text-right">
                                <div className={`text-xs font-bold ${ep.surprisal > 0.5 ? 'text-terracotta' : 'text-charcoal/60'}`}>
                                    S: {ep.surprisal}
                                </div>
                            </div>
                        </div>
                    ))}
                </div>
            </div>
        </div>
    );
}
