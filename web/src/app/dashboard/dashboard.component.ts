import { Component, OnInit, OnDestroy } from '@angular/core';
import { Subscription } from 'rxjs';
import { switchMap, catchError } from 'rxjs/operators';
import { of } from 'rxjs';
import { VisualizerService, ClusterSummary } from '../visualizer.service';
import { RefreshService } from '../refresh.service';

@Component({
  selector: 'app-dashboard',
  templateUrl: './dashboard.component.html',
  styleUrls: ['./dashboard.component.scss']
})
export class DashboardComponent implements OnInit, OnDestroy {

  summary: ClusterSummary | null = null;
  isLoading = true;
  private sub!: Subscription;

  constructor(
    private visualizerService: VisualizerService,
    private refreshService: RefreshService,
  ) { }

  ngOnInit(): void {
    this.sub = this.refreshService.tick$.pipe(
      switchMap(() => this.visualizerService.getClusterSummary().pipe(
        catchError(() => of(null))
      ))
    ).subscribe(data => {
      if (data) {
        this.summary = data;
      }
      this.isLoading = false;
    });
  }

  ngOnDestroy(): void {
    this.sub?.unsubscribe();
  }
}
