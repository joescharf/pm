import { useState } from "react";
import { Check, X, Plus, Trash2, Send } from "lucide-react";
import { useCreateReview, type CreateReviewInput } from "@/hooks/use-reviews";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import type { ReviewVerdict, ReviewCategory } from "@/lib/types";

const categoryOptions: { value: ReviewCategory; label: string }[] = [
  { value: "pass", label: "Pass" },
  { value: "fail", label: "Fail" },
  { value: "skip", label: "Skip" },
  { value: "na", label: "N/A" },
];

const categoryColors: Record<ReviewCategory, string> = {
  pass: "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300 border-green-300 dark:border-green-700",
  fail: "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border-red-300 dark:border-red-700",
  skip: "bg-gray-100 text-gray-800 dark:bg-gray-900/40 dark:text-gray-300 border-gray-300 dark:border-gray-600",
  na: "bg-gray-100 text-gray-800 dark:bg-gray-900/40 dark:text-gray-300 border-gray-300 dark:border-gray-600",
};

function CategorySelector({
  label,
  value,
  onChange,
}: {
  label: string;
  value: ReviewCategory;
  onChange: (v: ReviewCategory) => void;
}) {
  return (
    <div className="space-y-1.5">
      <Label className="text-xs text-muted-foreground">{label}</Label>
      <div className="flex gap-1">
        {categoryOptions.map((opt) => (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange(opt.value)}
            className={cn(
              "px-2 py-0.5 text-xs rounded-md border transition-colors",
              value === opt.value
                ? categoryColors[opt.value]
                : "border-transparent text-muted-foreground hover:bg-muted",
            )}
          >
            {opt.label}
          </button>
        ))}
      </div>
    </div>
  );
}

interface ReviewFormProps {
  issueId: string;
  defaultExpanded?: boolean;
}

export function ReviewForm({ issueId, defaultExpanded = false }: ReviewFormProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const [verdict, setVerdict] = useState<ReviewVerdict | null>(null);
  const [summary, setSummary] = useState("");
  const [codeQuality, setCodeQuality] = useState<ReviewCategory>("skip");
  const [requirementsMatch, setRequirementsMatch] = useState<ReviewCategory>("skip");
  const [testCoverage, setTestCoverage] = useState<ReviewCategory>("skip");
  const [uiUx, setUiUx] = useState<ReviewCategory>("na");
  const [failureReasons, setFailureReasons] = useState<string[]>([]);
  const [newReason, setNewReason] = useState("");

  const createReview = useCreateReview(issueId);

  const resetForm = () => {
    setVerdict(null);
    setSummary("");
    setCodeQuality("skip");
    setRequirementsMatch("skip");
    setTestCoverage("skip");
    setUiUx("na");
    setFailureReasons([]);
    setNewReason("");
  };

  const handleSubmit = () => {
    if (!verdict) {
      toast.error("Please select a verdict (Pass or Fail)");
      return;
    }
    if (!summary.trim()) {
      toast.error("Summary is required");
      return;
    }

    const input: CreateReviewInput = {
      verdict,
      summary: summary.trim(),
      code_quality: codeQuality,
      requirements_match: requirementsMatch,
      test_coverage: testCoverage,
      ui_ux: uiUx,
    };

    if (verdict === "fail" && failureReasons.length > 0) {
      input.failure_reasons = failureReasons;
    }

    createReview.mutate(input, {
      onSuccess: () => {
        toast.success(
          verdict === "pass"
            ? "Review submitted — issue closed"
            : "Review submitted — issue moved to in progress",
        );
        resetForm();
        setExpanded(false);
      },
      onError: (err) => {
        toast.error(`Failed to submit review: ${(err as Error).message}`);
      },
    });
  };

  const addReason = () => {
    const r = newReason.trim();
    if (r) {
      setFailureReasons((prev) => [...prev, r]);
      setNewReason("");
    }
  };

  const removeReason = (index: number) => {
    setFailureReasons((prev) => prev.filter((_, i) => i !== index));
  };

  if (!expanded) {
    return (
      <Card>
        <CardContent className="py-4">
          <Button variant="outline" size="sm" onClick={() => setExpanded(true)}>
            <Send className="size-4" />
            Submit Review
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Submit Review</CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {/* Verdict */}
        <div className="space-y-2">
          <Label>Verdict</Label>
          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setVerdict("pass")}
              className={cn(
                verdict === "pass" &&
                  "bg-green-100 text-green-800 border-green-300 dark:bg-green-900/40 dark:text-green-300 dark:border-green-700",
              )}
            >
              <Check className="size-4" />
              Pass
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setVerdict("fail")}
              className={cn(
                verdict === "fail" &&
                  "bg-red-100 text-red-800 border-red-300 dark:bg-red-900/40 dark:text-red-300 dark:border-red-700",
              )}
            >
              <X className="size-4" />
              Fail
            </Button>
          </div>
        </div>

        {/* Summary */}
        <div className="space-y-2">
          <Label htmlFor="review-summary">Summary</Label>
          <Textarea
            id="review-summary"
            placeholder="Describe the review findings..."
            value={summary}
            onChange={(e) => setSummary(e.target.value)}
            className="min-h-24"
          />
        </div>

        {/* Categories */}
        <div className="space-y-2">
          <Label className="text-sm">Categories</Label>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <CategorySelector
              label="Code Quality"
              value={codeQuality}
              onChange={setCodeQuality}
            />
            <CategorySelector
              label="Requirements"
              value={requirementsMatch}
              onChange={setRequirementsMatch}
            />
            <CategorySelector
              label="Test Coverage"
              value={testCoverage}
              onChange={setTestCoverage}
            />
            <CategorySelector label="UI/UX" value={uiUx} onChange={setUiUx} />
          </div>
        </div>

        {/* Failure Reasons (only when verdict is fail) */}
        {verdict === "fail" && (
          <div className="space-y-2">
            <Label>Failure Reasons</Label>
            {failureReasons.length > 0 && (
              <div className="space-y-1.5">
                {failureReasons.map((reason, i) => (
                  <div key={i} className="flex items-center gap-2">
                    <Badge variant="outline" className="flex-1 justify-start font-normal text-sm py-1">
                      {reason}
                    </Badge>
                    <button
                      type="button"
                      onClick={() => removeReason(i)}
                      className="text-muted-foreground hover:text-destructive transition-colors"
                    >
                      <Trash2 className="size-3.5" />
                    </button>
                  </div>
                ))}
              </div>
            )}
            <div className="flex gap-2">
              <Input
                placeholder="Add a failure reason..."
                value={newReason}
                onChange={(e) => setNewReason(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    addReason();
                  }
                }}
              />
              <Button type="button" variant="outline" size="sm" onClick={addReason} disabled={!newReason.trim()}>
                <Plus className="size-4" />
              </Button>
            </div>
          </div>
        )}

        {/* Actions */}
        <div className="flex gap-2 pt-2">
          <Button
            onClick={handleSubmit}
            disabled={createReview.isPending || !verdict || !summary.trim()}
          >
            <Send className="size-4" />
            {createReview.isPending ? "Submitting..." : "Submit Review"}
          </Button>
          <Button
            variant="ghost"
            onClick={() => {
              resetForm();
              setExpanded(false);
            }}
          >
            Cancel
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
