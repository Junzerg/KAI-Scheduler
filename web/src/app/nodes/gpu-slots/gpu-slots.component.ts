import { Component, Input, OnInit } from '@angular/core';
import { GPUSlot } from '../../visualizer.service';

@Component({
  selector: 'app-gpu-slots',
  templateUrl: './gpu-slots.component.html',
  styleUrls: ['./gpu-slots.component.scss']
})
export class GpuSlotsComponent implements OnInit {

  @Input() slots: GPUSlot[] = [];

  constructor() { }

  ngOnInit(): void {
  }

  getSlotClass(slot: GPUSlot): string {
    if (slot.occupiedBy) {
      return 'occupied';
    }
    if (slot.fragmented) {
      return 'fragmented';
    }
    return 'free';
  }

  getTooltip(slot: GPUSlot): string {
    let status = `GPU #${slot.id}`;
    if (slot.occupiedBy) {
      status += ` - Occupied by: ${slot.occupiedBy}`;
    } else if (slot.fragmented) {
      status += ' - Fragmented (Unusable due to constraints)';
    } else {
      status += ' - Free';
    }
    return status;
  }
}
