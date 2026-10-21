import {
  createContext,
  useContext,
  type AnchorHTMLAttributes,
  type MouseEvent,
  type ReactNode,
  type Ref,
} from 'react';

// Plain links for the browsing pages (P-10: no react-aria on the start path). A same-origin path
// is navigated by the app router on a plain left click; modified clicks (new tab, new window),
// other targets, downloads (`download`) and external URLs keep the browser's default. Enter on a
// focused link is a native click, so keyboard and spatial navigation work unchanged.

type Navigate = (href: string) => void;

const NavigateContext = createContext<Navigate | null>(null);

/** Hands the app router's navigate to every AppLink below it (set once in the root layout). */
export function LinkRouterProvider({
  navigate,
  children,
}: {
  navigate: Navigate;
  children: ReactNode;
}) {
  return <NavigateContext.Provider value={navigate}>{children}</NavigateContext.Provider>;
}

const isAppPath = (href: string) => href.startsWith('/') && !href.startsWith('//');

interface AppLinkProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  href: string;
  children?: ReactNode;
  ref?: Ref<HTMLAnchorElement>;
}

/** An `<a href>` that navigates inside the app without a reload. */
export function AppLink({ href, target, onClick, children, ref, ...rest }: AppLinkProps) {
  const navigate = useContext(NavigateContext);
  const handleClick = (event: MouseEvent<HTMLAnchorElement>) => {
    onClick?.(event);
    if (
      !navigate ||
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey ||
      (target !== undefined && target !== '_self') ||
      rest.download !== undefined ||
      !isAppPath(href)
    ) {
      return;
    }
    event.preventDefault();
    navigate(href);
  };
  return (
    <a ref={ref} href={href} target={target} onClick={handleClick} {...rest}>
      {children}
    </a>
  );
}
