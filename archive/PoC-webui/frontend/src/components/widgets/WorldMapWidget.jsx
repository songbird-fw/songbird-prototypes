import React from 'react';
import Widget from './Widget';
import { Globe } from 'lucide-react';

const WorldMapWidget = () => {
  return (
    <Widget title="Blocked Event Locations" icon={Globe}>
      <div className="p-2 h-full min-h-[160px] flex items-center justify-center bg-slate-900/[0.02] relative overflow-hidden">
        {/* Simple SVG World Map placeholder with threat hotspots */}
        <svg viewBox="0 0 1000 500" className="w-full h-full opacity-10">
          <path fill="#64748b" d="M150,150 L200,100 L250,150 L200,200 Z M400,300 L450,250 L500,300 L450,350 Z M700,100 L750,50 L800,100 L750,150 Z" />
          <circle cx="200" cy="150" r="100" fill="#64748b" />
          <circle cx="500" cy="300" r="120" fill="#64748b" />
          <circle cx="800" cy="150" r="80" fill="#64748b" />
        </svg>

        {/* Threat Hotspots */}
        <div className="absolute top-[30%] left-[20%] w-3 h-3 bg-red-500/20 rounded-full animate-ping"></div>
        <div className="absolute top-[30%] left-[20%] w-1.5 h-1.5 bg-red-600 rounded-full"></div>

        <div className="absolute top-[60%] left-[50%] w-5 h-5 bg-red-500/20 rounded-full animate-ping"></div>
        <div className="absolute top-[60%] left-[50%] w-2.5 h-2.5 bg-red-600 rounded-full"></div>

        <div className="absolute top-[40%] left-[75%] w-2 h-2 bg-red-500/20 rounded-full animate-ping"></div>
        <div className="absolute top-[40%] left-[75%] w-1 h-1 bg-red-600 rounded-full"></div>

        <div className="absolute bottom-1 right-1 flex flex-col gap-0.5">
          <div className="flex items-center gap-1">
            <div className="w-1 h-1 rounded-full bg-red-600"></div>
            <span className="text-[7px] font-bold text-slate-400 uppercase">Threat Active</span>
          </div>
        </div>
      </div>
    </Widget>
  );
};

export default WorldMapWidget;
