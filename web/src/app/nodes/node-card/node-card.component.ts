import { Component, Input, OnInit } from '@angular/core';
import { NodeView } from '../../visualizer.service';

@Component({
  selector: 'app-node-card',
  templateUrl: './node-card.component.html',
  styleUrls: ['./node-card.component.scss']
})
export class NodeCardComponent implements OnInit {

  @Input() node!: NodeView;

  constructor() { }

  ngOnInit(): void {
  }

  get isReady(): boolean {
    return this.node.status === 'Ready';
  }

  get cpuCapacity(): string {
    return (this.node.allocatable.milliCPU / 1000).toFixed(1) + ' Cores';
  }

  get memCapacity(): string {
    return (this.node.allocatable.memory / (1024 * 1024 * 1024)).toFixed(1) + ' GiB';
  }

  get cpuUsagePercent(): number {
    if (!this.node.allocatable.milliCPU) return 0;
    return (this.node.used.milliCPU / this.node.allocatable.milliCPU) * 100;
  }

  get memUsagePercent(): number {
    if (!this.node.allocatable.memory) return 0;
    return (this.node.used.memory / this.node.allocatable.memory) * 100;
  }

  get hasGpu(): boolean {
    return this.node.gpuSlots && this.node.gpuSlots.length > 0;
  }
}
