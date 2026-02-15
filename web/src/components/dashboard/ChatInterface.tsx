"use client";

import { useState, useEffect, useRef } from "react";
import { motion } from "framer-motion";
import { Send, Sparkles, Bot, User, Settings, CircleDashed } from "lucide-react";
import { cmaApi } from "@/lib/cma-api";

interface Message {
    id: string;
    role: "user" | "assistant";
    content: string;
    thinking?: boolean;
}

export default function ChatInterface() {
    const [messages, setMessages] = useState<Message[]>([
        {
            id: "welcome",
            role: "assistant",
            content: "Hello. I am Memora. I have access to your episodic memories and semantic knowledge graph. How can I help you today?"
        }
    ]);
    const [input, setInput] = useState("");
    const [isLoading, setIsLoading] = useState(false);
    const scrollRef = useRef<HTMLDivElement>(null);
    const userId = "harsh1"; // Hardcoded for now

    useEffect(() => {
        if (scrollRef.current) {
            scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
        }
    }, [messages]);

    const handleSend = async () => {
        if (!input.trim() || isLoading) return;

        const userMsg: Message = { id: Date.now().toString(), role: "user", content: input };
        setMessages(prev => [...prev, userMsg]);
        setInput("");
        setIsLoading(true);

        try {
            // 1. Ingest User Message
            await cmaApi.ingest(userId, userMsg.content, "user");

            // 2. Query for response
            const response = await cmaApi.query(userId, userMsg.content);

            // 3. Ingest Assistant Response (to maintain history in backend)
            await cmaApi.ingest(userId, response.context, "assistant");

            const botMsg: Message = {
                id: (Date.now() + 1).toString(),
                role: "assistant",
                content: response.context,
            };
            setMessages(prev => [...prev, botMsg]);

        } catch (error) {
            console.error("Chat error:", error);
            const errorMsg: Message = {
                id: (Date.now() + 1).toString(),
                role: "assistant",
                content: "I encountered an error connecting to the Cognitive Architecture. Please ensure the backend is running.",
            };
            setMessages(prev => [...prev, errorMsg]);
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div className="h-full flex flex-col font-sans relative">
            {/* Header */}
            <div className="flex items-center justify-between mb-4 border-b border-terracotta/10 pb-4 shrink-0">
                <div>
                    <h2 className="text-xl font-bold text-terracotta tracking-tight flex items-center gap-2">
                        <Sparkles className="text-beige" size={20} />
                        Wake Mode
                    </h2>
                </div>
                <div className="flex gap-2">
                    <button className="p-2 text-terracotta/70 hover:text-terracotta hover:bg-terracotta/10 rounded-lg transition-colors">
                        <Settings size={18} />
                    </button>
                </div>
            </div>

            {/* Chat Area */}
            <div className="flex-1 overflow-y-auto space-y-6 pr-2 custom-scrollbar" ref={scrollRef}>
                {messages.map((msg) => (
                    <div key={msg.id} className={`flex gap-4 ${msg.role === "user" ? "flex-row-reverse" : ""}`}>
                        <div className={`w-8 h-8 rounded-full flex items-center justify-center shrink-0 shadow-sm ${msg.role === "assistant" ? "bg-gradient-to-br from-terracotta to-charcoal" : "bg-beige"}`}>
                            {msg.role === "assistant" ? <Bot size={16} className="text-white" /> : <User size={16} className="text-charcoal" />}
                        </div>
                        <div className={`space-y-2 max-w-3xl ${msg.role === "user" ? "flex justify-end" : ""}`}>
                            <div className={`text-sm leading-relaxed border p-3 rounded-2xl shadow-sm ${msg.role === "assistant"
                                ? "text-charcoal/90 bg-white border-terracotta/20 rounded-tl-none"
                                : "text-charcoal bg-cream/50 border-terracotta/20 rounded-tr-none"
                                }`}>
                                <p className="whitespace-pre-wrap">{msg.content}</p>
                            </div>
                        </div>
                    </div>
                ))}

                {isLoading && (
                    <div className="flex gap-4">
                        <div className="w-8 h-8 rounded-full bg-gradient-to-br from-terracotta to-charcoal flex items-center justify-center shrink-0 shadow-sm">
                            <Bot size={16} className="text-white" />
                        </div>
                        <div className="space-y-2">
                            <motion.div
                                initial={{ opacity: 0, height: 0 }}
                                animate={{ opacity: 1, height: 'auto' }}
                                className="flex items-center gap-2 text-[10px] font-mono text-terracotta/80 py-1"
                            >
                                <CircleDashed size={12} className="animate-spin text-terracotta" />
                                <span>Generating...</span>
                            </motion.div>
                        </div>
                    </div>
                )}
            </div>

            {/* Input Area - Pinned to bottom */}
            <div className="pt-4 mt-2 shrink-0">
                <div className="relative">
                    <input
                        type="text"
                        value={input}
                        onChange={(e) => setInput(e.target.value)}
                        onKeyDown={(e) => e.key === "Enter" && handleSend()}
                        placeholder="Message Memora..."
                        className="w-full bg-transparent border-b border-terracotta/20 px-0 py-3 pr-12 text-sm text-charcoal placeholder-charcoal/40 focus:outline-none focus:border-terracotta transition-all"
                        disabled={isLoading}
                    />
                    <button
                        onClick={handleSend}
                        disabled={isLoading}
                        className="absolute right-0 top-2 p-1.5 text-terracotta/60 hover:text-terracotta transition-colors cursor-pointer disabled:opacity-50"
                    >
                        <Send size={18} />
                    </button>
                </div>
                <div className="text-[10px] text-center text-charcoal/30 mt-2">
                    Memora can make mistakes. Check important info.
                </div>
            </div>
        </div>
    );
}
