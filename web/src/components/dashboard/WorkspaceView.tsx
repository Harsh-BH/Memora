"use client";

import { useState, useEffect } from "react";
import { Cpu, CheckCircle, XCircle, RefreshCw } from "lucide-react";
import clsx from "clsx";
import { cmaApi } from "@/lib/cma-api";

export default function WorkspaceView() {
    const [data, setData] = useState<any>(null);
    const [loading, setLoading] = useState(true);
    const userId = "harsh1";

    const fetchData = async () => {
        try {
            const res = await cmaApi.getWorkspaceContext(userId);
            setData(res);
        } catch (error) {
            console.error(error);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchData();
        const interval = setInterval(fetchData, 5000);
        return () => clearInterval(interval);
    }, []);

    const items = data?.items || [];
    const totalTokens = data?.total_tokens || 0;
    const tokenBudget = data?.token_budget || 4096;
    const usagePercent = Math.min((totalTokens / tokenBudget) * 100, 100);

    return (
        <div className="space-y-6">
            <header>
                <h2 className="text-2xl font-bold text-terracotta tracking-tight flex items-center gap-2">
                    <Cpu className="text-beige" size={24} />
                    Cognitive Workspace
                    {loading && <RefreshCw size={14} className="animate-spin ml-2 opacity-50" />}
                </h2>
                <p className="text-charcoal/60 text-sm">Context window optimization using DIG reranking and Knapsack algorithm.</p>
            </header>

            {/* Token Usage Bar */}
            <div className="bg-white p-6 rounded-2xl border border-terracotta/20 shadow-sm">
                <div className="flex justify-between items-center mb-4">
                    <div>
                        <h3 className="font-semibold text-terracotta">Token Budget Usage</h3>
                        <p className="text-xs text-charcoal/60">Optimization Goal: Maximize Information Density</p>
                    </div>
                    <div className="text-right">
                        <span className="text-2xl font-bold text-terracotta">{totalTokens.toLocaleString()}</span>
                        <span className="text-sm text-charcoal/60"> / {tokenBudget.toLocaleString()}</span>
                    </div>
                </div>
                <div className="w-full h-4 bg-cream rounded-full overflow-hidden">
                    <div
                        className="h-full bg-gradient-to-r from-beige to-terracotta transition-all duration-500"
                        style={{ width: `${usagePercent}%` }}
                    />
                </div>
            </div>

            {/* Knapsack List */}
            <div className="bg-white rounded-2xl border border-terracotta/20 shadow-sm overflow-hidden">
                <div className="p-4 border-b border-terracotta/20 flex justify-between items-center bg-cream/50">
                    <h3 className="font-semibold text-terracotta text-sm">Candidate Memories</h3>
                    <span className="text-[10px] font-mono text-charcoal/60">Sort: Density (DIG/Weight)</span>
                </div>
                <div className="divide-y divide-terracotta/10 max-h-[400px] overflow-y-auto custom-scrollbar">
                    {items.map((item: any) => (
                        <div key={item.id} className={clsx(
                            "p-4 flex items-center justify-between transition-colors",
                            item.status === "rejected" ? "bg-cream/50 opacity-60" : "hover:bg-terracotta/5"
                        )}>
                            <div className="flex items-start gap-3">
                                <div className={clsx(
                                    "mt-1",
                                    item.status === "kept" ? "text-terracotta" : "text-beige"
                                )}>
                                    {item.status === "kept" ? <CheckCircle size={18} /> : <XCircle size={18} />}
                                </div>
                                <div>
                                    <p className={clsx(
                                        "text-sm font-medium",
                                        item.status === "rejected" ? "text-charcoal/40 line-through" : "text-charcoal"
                                    )}>
                                        {item.content}
                                    </p>
                                    <div className="flex items-center gap-3 mt-1">
                                        <span className="text-[10px] bg-cream text-charcoal/60 px-1.5 py-0.5 rounded font-mono">
                                            {item.weight} toks
                                        </span>
                                        <span className={clsx(
                                            "text-[10px] font-mono",
                                            item.value > 0.5 ? "text-terracotta" : "text-beige"
                                        )}>
                                            DIG: {item.value}
                                        </span>
                                    </div>
                                </div>
                            </div>
                            <div>
                                {item.status === "kept" ? (
                                    <span className="text-xs font-bold text-terracotta border border-terracotta/30 bg-white px-2 py-1 rounded">KEPT</span>
                                ) : (
                                    <span className="text-xs font-semibold text-charcoal/60">DROP</span>
                                )}
                            </div>
                        </div>
                    ))}
                    {items.length === 0 && !loading && (
                        <div className="p-8 text-center text-charcoal/40 text-sm">
                            <Cpu size={24} className="mx-auto mb-2 opacity-20" />
                            Workspace empty.
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
