import React from 'react';
import Widget from './Widget';
import { Cpu } from 'lucide-react';

const GaugeWidget = ({ title, value, icon: Icon = Cpu, color = "bg-blue-500" }) => {
  const radius = 40;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (value / 100) * circumference;

  return (
    <Widget title={title} icon={Icon}>
      <div className="p-4 flex flex-col items-center justify-center h-full">
        <div className="relative w-24 h-24">
          <svg className="w-full h-full transform -rotate-90">
            <circle
              cx="48"
              cy="48"
              r={radius}
              stroke="currentColor"
              strokeWidth="8"
              fill="transparent"
              className="text-slate-100"
            />
            <circle
              cx="48"
              cy="48"
              r={radius}
              stroke="currentColor"
              strokeWidth="8"
              fill="transparent"
              strokeDasharray={circumference}
              strokeDashoffset={offset}
              strokeLinecap="round"
              className={`${color.replace('bg-', 'text-')} transition-all duration-500`}
            />
          </svg>
          <div className="absolute inset-0 flex flex-col items-center justify-center">
            <span className="text-lg font-black text-slate-700">{value}%</span>
          </div>
        </div>
      </div>
    </Widget>
  );
};

export default GaugeWidget;
