import React from 'react';
import Widget from './Widget';
import { Globe } from 'lucide-react';

const WorldMapWidget = () => {
  return (
    <Widget title="Blocked Event Locations" icon={Globe}>
      <div className="p-4 h-full min-h-[140px] flex items-center justify-center bg-slate-900/5 relative overflow-hidden">
        {/* Simple SVG World Map placeholder with threat hotspots */}
        <svg viewBox="0 0 1000 500" className="w-full h-full opacity-20">
          <path fill="#94a3b8" d="M150,150 L200,100 L250,150 L200,200 Z M400,300 L450,250 L500,300 L450,350 Z M700,100 L750,50 L800,100 L750,150 Z" />
          <circle cx="200" cy="150" r="100" fill="#94a3b8" />
          <circle cx="500" cy="300" r="120" fill="#94a3b8" />
          <circle cx="800" cy="150" r="80" fill="#94a3b8" />
        </svg>

        {/* Threat Hotspots */}
        <div className="absolute top-[30%] left-[20%] w-4 h-4 bg-red-500 rounded-full animate-ping"></div>
        <div className="absolute top-[30%] left-[20%] w-2 h-2 bg-red-600 rounded-full"></div>

        <div className="absolute top-[60%] left-[50%] w-6 h-6 bg-red-500 rounded-full animate-ping"></div>
        <div className="absolute top-[60%] left-[50%] w-3 h-3 bg-red-600 rounded-full"></div>

        <div className="absolute top-[40%] left-[75%] w-3 h-3 bg-red-500 rounded-full animate-ping"></div>
        <div className="absolute top-[40%] left-[75%] w-1.5 h-1.5 bg-red-600 rounded-full"></div>

        <div className="absolute bottom-2 right-2 flex flex-col gap-1">
          <div className="flex items-center gap-1.5">
            <div className="w-2 h-2 rounded-full bg-red-600"></div>
            <span className="text-[8px] font-bold text-slate-500 uppercase">High Threat</span>
          </div>
        </div>
      </div>
    </Widget>
  );
};

export default WorldMapWidget;
