import React from 'react';
import { Activity, Cpu, Database, HardDrive, Zap } from 'lucide-react';
import GaugeWidget from '../components/widgets/GaugeWidget';
import Widget from '../components/widgets/Widget';

const SystemHealth = () => {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-slate-900">System Health</h1>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <GaugeWidget title="CPU Usage" value={24} icon={Cpu} color="bg-blue-500" />
        <GaugeWidget title="Memory Usage" value={42} icon={Activity} color="bg-green-500" />
        <GaugeWidget title="Disk Utilization" value={18} icon={HardDrive} color="bg-amber-500" />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        <div className="lg:col-span-6">
           <Widget title="Services Performance" icon={Database}>
             <div className="p-4 space-y-4">
                <div className="space-y-1">
                   <div className="flex justify-between text-[10px] font-bold uppercase text-slate-500">
                      <span>Firewall Engine Load</span>
                      <span>Low</span>
                   </div>
                   <div className="h-1.5 w-full bg-slate-100 rounded-full overflow-hidden">
                      <div className="h-full bg-green-500 w-[15%]"></div>
                   </div>
                </div>
                <div className="space-y-1">
                   <div className="flex justify-between text-[10px] font-bold uppercase text-slate-500">
                      <span>Log Indexer</span>
                      <span>Optimal</span>
                   </div>
                   <div className="h-1.5 w-full bg-slate-100 rounded-full overflow-hidden">
                      <div className="h-full bg-blue-500 w-[35%]"></div>
                   </div>
                </div>
             </div>
           </Widget>
        </div>

        <div className="lg:col-span-6">
           <Widget title="Storage Status" icon={HardDrive}>
              <div className="p-4 text-sm space-y-3">
                 <div className="flex justify-between">
                    <span className="text-slate-500">Logs Partition</span>
                    <span className="font-bold">4.2 GB / 50 GB</span>
                 </div>
                 <div className="flex justify-between">
                    <span className="text-slate-500">System Backup</span>
                    <span className="font-bold">1.1 GB</span>
                 </div>
              </div>
           </Widget>
        </div>
      </div>
    </div>
  );
};

export default SystemHealth;
