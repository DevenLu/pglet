import React from 'react'
import { shallowEqual, useSelector } from 'react-redux'
import ControlsList from './ControlsList'
import useTitle from '../hooks/useTitle'
import { Stack } from 'office-ui-fabric-react/lib/Stack';

const Page = React.memo(({ control }) => {

  console.log(`render page: ${control.i}`);

  // page title
  let title = "Pglet";
  if (control.title) {
    title = control.title
  }
  useTitle(title)

  // stack props
  const stackProps = {
    verticalFill: control.verticalFill ? control.verticalFill : false,
    horizontalAlign: control.horizontalalign ? control.horizontalalign : "start",
    verticalAlign: control.verticalalign ? control.verticalalign : "start",
    styles: {
      root: {
        width: control.width ? control.width : "100%",
        padding: control.padding ? control.padding : "10px"
      }
    },
  };

  const stackTokens = {
    childrenGap: control.gap ? control.gap : 10
  }

  const childControls = useSelector(state => control.c.map(childId => state.page.controls[childId]), shallowEqual);

  return <Stack tokens={stackTokens} {...stackProps}>
    <ControlsList controls={childControls} />
  </Stack>
})

export default Page