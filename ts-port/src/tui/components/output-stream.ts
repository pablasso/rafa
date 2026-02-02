/**
 * Claude output display component
 */

export class OutputStreamComponent {
  private content: string = "";

  append(text: string): void {
    this.content += text;
  }

  clear(): void {
    this.content = "";
  }

  getContent(): string {
    return this.content;
  }

  render(_width: number): string[] {
    return ["Output Stream - placeholder"];
  }
}
