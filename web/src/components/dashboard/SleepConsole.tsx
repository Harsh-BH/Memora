"use client";

import { Terminal, Moon, RefreshCw } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { cmaApi } from "@/lib/cma-api";

export default function SleepConsole() {
    const [logs, setLogs] = useState<{ ts: string, level: string, module: string, msg: string }[]>([]);
    const bottomRef = useRef<HTMLDivElement>(null);
    const [connected, setConnected] = useState(false);

    const fetchLogs = async () => {
        try {
            const data = await cmaApi.getSystemLogs();
            if (data && data.length > 0) {
                // Determine if we need to scroll (only if near bottom or first load)
                // For now, auto-scroll if new logs come in
                setLogs(data);
                setConnected(true);
            }
        } catch (error) {
            console.error("Failed to fetch logs", error);
            setConnected(false);
        }
    };

    useEffect(() => {
        fetchLogs();
        const interval = setInterval(fetchLogs, 2000); // Poll every 2s for "live" feel
        return () => clearInterval(interval);
    }, []);

    useEffect(() => {
        bottomRef.current?.scrollIntoView({ behavior: "smooth" });
    }, [logs]);

    return (
        <div className="h-full flex flex-col space-y-6">
            <header className="shrink-0">
                <h2 className="text-2xl font-bold text-terracotta tracking-tight flex items-center gap-2">
                    <Moon className="text-beige" size={24} />
                    Sleep Console
                    {!connected && <RefreshCw size={14} className="animate-spin ml-2 opacity-50 text-red-500" />}
                </h2>
                <p className="text-charcoal/60 text-sm">Background workers, consolidation logs, and system health.</p>
            </header>

            <div className="flex-1 bg-charcoal rounded-2xl border border-white/10 shadow-lg p-6 font-mono text-sm overflow-hidden flex flex-col">
                <div className="flex items-center gap-2 border-b border-white/10 pb-4 mb-4">
                    <Terminal size={16} className="text-terracotta" />
                    <span className="text-terracotta">cma-worker-01</span>
                    <span className={`ml-auto flex h-2 w-2 rounded-full animate-pulse ${connected ? 'bg-terracotta' : 'bg-red-500'}`} />
                </div>

                <div className="flex-1 overflow-y-auto space-y-2 pr-2 custom-scrollbar">
                    {logs.length === 0 && (
                        <div className="text-white/30 italic">No logs received yet. Waiting for system activity...</div>
                    )}
                    {logs.map((log, i) => (
                        <div key={i} className="flex gap-3 text-cream/90">
                            <span className="text-white/40 opacity-50 select-none">{log.ts}</span>
                            <span className={`font-bold w-12 shrink-0 ${log.level === 'INFO' ? 'text-terracotta' :
                                log.level === 'WARN' ? 'text-beige' :
                                    log.level === 'DEBUG' ? 'text-white/60' :
                                        log.level === 'ERROR' ? 'text-red-400' : 'text-slate-300'
                                }`}>
                                {log.level}
                            </span>
                            <span className="text-white/40 w-24 shrink-0">[{log.module}]</span>
                            <span>{log.msg}</span>
                        </div>
                    ))}
                    <div className="h-4" ref={bottomRef} />
                </div>
            </div>
        </div>
    );
}
