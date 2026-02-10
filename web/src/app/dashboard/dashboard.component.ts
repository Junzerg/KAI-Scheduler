import { Component, OnInit } from '@angular/core';
import { Observable } from 'rxjs';
import { VisualizerService, ClusterSummary } from '../visualizer.service';

@Component({
  selector: 'app-dashboard',
  templateUrl: './dashboard.component.html',
  styleUrls: ['./dashboard.component.scss']
})
export class DashboardComponent implements OnInit {

  summary$: Observable<ClusterSummary> | undefined;

  constructor(private visualizerService: VisualizerService) { }

  ngOnInit(): void {
    // Basic polling or just fetch once for now.
    // In Phase 2.5 we will implement auto-refresh.
    this.summary$ = this.visualizerService.getClusterSummary();
  }

}
